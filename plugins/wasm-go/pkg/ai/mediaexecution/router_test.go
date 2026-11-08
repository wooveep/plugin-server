package mediaexecution_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

func TestRouterMediaProfilesRetainTheirConsumedSenderAndSingleSend(t *testing.T) {
	for _, profile := range []string{mediaexecution.Profile, mediaexecution.QwenImage20ProSync, mediaexecution.QwenImage30NativeSync, mediaexecution.QwenImage30Compatible, mediaexecution.Wan30Text, mediaexecution.Wan30FirstFrame} {
		t.Run(profile, func(t *testing.T) {
			f := profileFixture(t, profile)
			f.candidate["send_transport"], f.candidate["recording_owner"] = "router", "router_observer"
			var procurement map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw(t, f.archive["procurement"]), &procurement))
			procurement["writer"] = raw(t, map[string]string{"producer_id": f.gateway.ProducerID, "role": "router_observer"})
			f.archive["procurement"] = procurement
			f.history["delivery_writer"] = map[string]string{"producer_id": f.gateway.ProducerID, "role": "router_observer"}
			executor, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
			require.NoError(t, err)
			selection := f.selection(t, false)
			permission := resp.ArrayValue([]resp.Value{resp.StringValue("allow"), resp.StringValue("100-0")})
			request, err := executor.PrepareRedisCreate(permission, raw(t, selection), f.live, f.now)
			require.NoError(t, err)
			attempt, err := request.NewAttempt(f.gateway.ProducerID)
			require.NoError(t, err)
			// The immutable source records the actual supplier Router send. Its
			// internal control RPC cannot change it back to an http_call attempt.
			observed, err := request.Observe(mediaexecution.Response{ObservedAt: f.now.Format(time.RFC3339Nano), SendState: "not_sent"})
			require.NoError(t, err)
			fact, err := attempt.Finish(observed.ActualAttempt)
			require.NoError(t, err)
			initial := fact.Envelope()
			require.Equal(t, "router", initial.SendTransport)
			require.Equal(t, "router_observer", initial.RecordingOwner)
			require.Equal(t, "POST", request.Method())
			_, err = executor.PrepareRedisCreate(permission, raw(t, selection), f.live, f.now)
			require.ErrorIs(t, err, mediaexecution.ErrReplay)
		})
	}
}
