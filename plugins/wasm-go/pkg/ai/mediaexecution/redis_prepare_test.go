package mediaexecution_test

import (
	"errors"
	"github.com/higress-group/wasm-go/pkg/ai/mediaexecution"
	"github.com/tidwall/resp"
	"testing"
	"time"
)

func TestRedisMediaFirstReplySealsOnlyOriginalCreate(t *testing.T) {
	for _, check := range []struct {
		name    string
		reply   resp.Value
		allowed bool
	}{
		{"first", resp.ArrayValue([]resp.Value{resp.StringValue("allow"), resp.StringValue("123-0")}), true},
		{"replay", resp.ArrayValue([]resp.Value{resp.StringValue("deny"), resp.StringValue("media_ticket_consumed")}), false},
		{"unknown", resp.ErrorValue(errors.New("lost Redis response")), false},
		{"not_a_receipt", resp.ArrayValue([]resp.Value{resp.StringValue("allow"), resp.StringValue("forged")}), false},
	} {
		t.Run(check.name, func(t *testing.T) {
			f := original(t)
			e, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
			if err != nil {
				t.Fatal(err)
			}
			selection, err := e.Selection(f.live, "", nil)
			if err != nil {
				t.Fatal(err)
			}
			request, err := e.PrepareRedisCreate(check.reply, selection, f.live, f.now)
			if (err == nil) != check.allowed {
				t.Fatalf("permission=%v err=%v", check.allowed, err)
			}
			if check.allowed {
				if request.AttemptID() != "attempt-create" || request.IsQuery() || request.Endpoint() != "https://vendor.example" {
					t.Fatal("original account or attempt changed")
				}
				if _, err := request.NewAttempt(f.gateway.ProducerID); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := e.PrepareRedisCreate(resp.ArrayValue([]resp.Value{resp.StringValue("allow"), resp.StringValue("124-0")}), selection, f.live, f.now); err == nil {
				t.Fatal("second reply or lost first reply recreated provider permission")
			}
		})
	}
}
func TestRedisMediaOriginalTaskQueryKeepsOriginalHistoryAndFiniteDeadline(t *testing.T) {
	f := original(t)
	f.archive["dispatch"] = f.dispatch("consumed")
	f.archive["history"] = f.sealed(f.selection(t, false), "task-original")
	e, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := e.Selection(f.live, "query-original", nil)
	if err != nil {
		t.Fatal(err)
	}
	reply := resp.ArrayValue([]resp.Value{resp.StringValue("allow"), resp.StringValue("123-0")})
	request, err := e.PrepareRedisQuery(reply, selection, f.now.Add(time.Second), f.now)
	if err != nil {
		t.Fatal(err)
	}
	if !request.IsQuery() || request.Path() != "/api/v1/tasks/task-original" || request.AttemptID() != "query-original" {
		t.Fatal("query substituted task or account")
	}
	if _, err := e.PrepareRedisQuery(reply, selection, f.now.Add(time.Hour), f.now); err == nil {
		t.Fatal("query replay renewed a send")
	}
}

func TestProjectionUnreadyRetainsOnlyTheOriginalUnconsumedCreate(t *testing.T) {
	f := original(t)
	e, err := mediaexecution.FromPrivateArchive(raw(t, f.archive), f.gateway, f.binding)
	if err != nil {
		t.Fatal(err)
	}
	selection, err := e.Selection(f.live, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	denied := resp.ArrayValue([]resp.Value{resp.StringValue("deny"), resp.StringValue("projection_unready")})
	for range 2 {
		if _, err := e.PrepareRedisCreate(denied, selection, f.live, f.now); !errors.Is(err, mediaexecution.ErrProjectionUnready) {
			t.Fatal("a definite unready denial consumed or changed the original permission")
		}
	}
	allowed := resp.ArrayValue([]resp.Value{resp.StringValue("allow"), resp.StringValue("123-0")})
	request, err := e.PrepareRedisCreate(allowed, selection, f.live, f.now)
	if err != nil || request.AttemptID() != "attempt-create" {
		t.Fatal("original readiness did not retain the original attempt", err)
	}
	if _, err := e.PrepareRedisCreate(denied, selection, f.live, f.now); !errors.Is(err, mediaexecution.ErrReplay) {
		t.Fatal("a later denial reopened a consumed permission")
	}
}
