//go:build integration

package stdio

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexIntegration_SingleTurn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	spec := CodexSpec{}
	proc, err := NewStdioProcess(ctx, spec, StartOptions{
		WorkDir: "/data/app/workspace/me/demo",
		Context: ctx,
	})
	require.NoError(t, err)
	defer proc.Close()

	// Write prompt via stdin, then close to signal EOF
	err = proc.WriteInput("Reply with exactly: HELLO_CODEx_OK")
	require.NoError(t, err)
	require.NoError(t, proc.CloseInput())

	// Collect events
	var result string
	var gotSessionID bool
	var done bool
	for evt := range proc.Events() {
		t.Logf("event: type=%d text=%q sessionID=%q done=%v err=%v",
			evt.Type, truncate(evt.Text, 80), evt.SessionID, evt.Done, evt.Error)

		switch evt.Type {
		case EventResult:
			result += evt.Text
			if evt.Done {
				done = true
			}
		case EventError:
			t.Logf("error event: %v", evt.Error)
		}

		if evt.SessionID != "" {
			gotSessionID = true
		}
	}

	assert.True(t, done, "should receive turn.completed")
	assert.True(t, gotSessionID, "should receive thread_id from thread.started")
	assert.True(t, strings.Contains(strings.ToUpper(result), "HELLO"), "result should contain HELLO")
	t.Logf("final result: %s", truncate(result, 200))
}

func TestCodexIntegration_ResumeTurn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	spec := CodexSpec{}

	// Turn 1: fresh
	ctx1, cancel1 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel1()

	proc1, err := NewStdioProcess(ctx1, spec, StartOptions{
		WorkDir: "/data/app/workspace/me/demo",
		Context: ctx1,
	})
	require.NoError(t, err)

	err = proc1.WriteInput("Remember the number 42. Reply with just: OK")
	require.NoError(t, err)
	require.NoError(t, proc1.CloseInput())

	var sessionID string
	for evt := range proc1.Events() {
		if evt.SessionID != "" {
			sessionID = evt.SessionID
		}
	}
	proc1.Close()
	require.NotEmpty(t, sessionID, "must capture session ID from turn 1")
	t.Logf("turn 1 sessionID: %s", sessionID)

	// Turn 2: resume
	ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel2()

	proc2, err := NewStdioProcess(ctx2, spec, StartOptions{
		WorkDir:   "/data/app/workspace/me/demo",
		Context:   ctx2,
		Resume:    true,
		SessionID: sessionID,
	})
	require.NoError(t, err)
	defer proc2.Close()

	// Verify resume args use SESSION_ID (not --last)
	args := spec.BuildArgs(StartOptions{Resume: true, SessionID: sessionID})
	t.Logf("resume args: %v", args)
	assert.Contains(t, args, "resume")
	assert.Contains(t, args, sessionID)

	err = proc2.WriteInput("What number did I ask you to remember? Reply with just the number.")
	require.NoError(t, err)
	require.NoError(t, proc2.CloseInput())

	var result2 string
	var done2 bool
	for evt := range proc2.Events() {
		t.Logf("turn2 event: type=%d text=%q sessionID=%q done=%v err=%v",
			evt.Type, truncate(evt.Text, 80), evt.SessionID, evt.Done, evt.Error)
		if evt.Type == EventResult && evt.Text != "" {
			result2 += evt.Text
		}
		if evt.Type == EventResult && evt.Done {
			done2 = true
		}
	}

	assert.True(t, done2, "turn 2 should complete")
	assert.Contains(t, result2, "42", "resume turn should recall the number 42")
	t.Logf("turn 2 result: %s", truncate(result2, 200))
}
