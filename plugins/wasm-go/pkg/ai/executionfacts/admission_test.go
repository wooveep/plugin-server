package executionfacts

import (
	"strings"
	"testing"
	"time"
)

func TestAdmittedCallPreservesOriginalTimeAndRejectsAmbiguousAuthority(t *testing.T) {
	at := time.Date(2026, 10, 28, 1, 2, 3, 123456000, time.UTC)
	headers := [][2]string{
		{AdmissionCallHeader, "10000000-0000-4000-8000-000000000001"},
		{AdmissionReleaseHeader, "original-release"},
		{AdmissionDeadlineHeader, at.Add(5 * time.Second).Format(time.RFC3339Nano)},
		{AdmissionOccurredHeader, "2026-10-28T01:02:03.123456000Z"},
	}
	call, err := ReadAdmittedCall(headers, "original-release", at)
	if err != nil || call.OccurredAt != headers[3][1] || call.CallID != headers[0][1] {
		t.Fatalf("original call identity was not retained: %#v, %v", call, err)
	}
	for index := range headers {
		missing := append([][2]string(nil), headers[:index]...)
		missing = append(missing, headers[index+1:]...)
		if _, err := ReadAdmittedCall(missing, "original-release", at); err == nil {
			t.Fatalf("missing field %s accepted", headers[index][0])
		}
		duplicate := append(append([][2]string(nil), headers...), [2]string{strings.ToUpper(headers[index][0]), headers[index][1]})
		if _, err := ReadAdmittedCall(duplicate, "original-release", at); err == nil {
			t.Fatalf("duplicate field %s accepted", headers[index][0])
		}
	}
	for _, tc := range []struct {
		index int
		value string
	}{
		{0, "client-request-id"}, {1, "different-release"},
		{2, at.Format(time.RFC3339Nano)},
		{3, at.Add(time.Second).Format(time.RFC3339Nano)},
		{3, "2026-10-28T01:02:03.123456+00:00"},
	} {
		changed := append([][2]string(nil), headers...)
		changed[tc.index][1] = tc.value
		if _, err := ReadAdmittedCall(changed, "original-release", at); err == nil {
			t.Fatalf("invalid original admission field %d accepted", tc.index)
		}
	}
}
