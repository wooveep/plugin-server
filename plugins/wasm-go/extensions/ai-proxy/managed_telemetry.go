package main

import (
	"github.com/higress-group/proxy-wasm-go-sdk/properties"
	"github.com/higress-group/wasm-go/pkg/ai/executionfacts"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"net"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func captureManagedClientInfo(headers [][2]string) executionfacts.ClientInfo {
	info := executionfacts.ClientInfo{}
	if agent, err := managedSingleHeader(headers, "user-agent"); err == nil && utf8.ValidString(agent) {
		valid := true
		for _, char := range agent {
			valid = valid && !unicode.IsControl(char)
		}
		if valid {
			if len(agent) > executionfacts.MaxUserAgentBytes {
				agent = agent[:executionfacts.MaxUserAgentBytes]
				for !utf8.ValidString(agent) {
					agent = agent[:len(agent)-1]
				}
			}
			info.UserAgent = agent
		}
	}
	// Via is only a suppression hint, never proof of a portal's original IP.
	// Console binds the native call ID to its own original observation.
	if via, err := managedSingleHeader(headers, "via"); err == nil {
		for _, hop := range strings.Split(via, ",") {
			if strings.EqualFold(strings.TrimSpace(hop), "1.1 modelfusion-portal") {
				info.ClientIPSource = executionfacts.ClientIPUnconfirmedPortal
				return info
			}
		}
	}
	address, err := properties.GetDownstreamRemoteAddress()
	if err != nil {
		return info
	}
	if host, _, err := net.SplitHostPort(address); err == nil {
		address = host
	}
	if ip := net.ParseIP(address); ip != nil {
		info.ClientIP = ip.String()
		info.ClientIPSource = executionfacts.ClientIPDownstreamConnection
	}
	return info
}

func recordManagedTelemetry(ctx wrapper.HttpContext, s *managedExecutionState, outcome executionfacts.AttemptOutcome) {
	if s == nil || s.attemptID == "" {
		return
	}
	sent := ""
	if outcome.SendState == "observed_sent" && !s.sendStartedAt.IsZero() {
		sent = s.sendStartedAt.UTC().Format(time.RFC3339Nano)
	}
	recordManagedAccessUpstream(ctx, s)
	snapshot, err := executionfacts.FreezeRuntimeTelemetry(executionfacts.RuntimeTelemetry{ClientInfo: s.client, ProducerID: s.config.settings.ProducerID, Kind: "attempt", CallID: s.callID, AttemptID: s.attemptID, RequestID: wrapper.PreserveOriginalRequestID(ctx), UpstreamRequestID: s.upstreamRequestID, SentAt: sent, FirstByteMS: s.firstByteMS, ObservedAt: managedNow()})
	if err == nil {
		dispatchManagedSnapshot(ctx, s, snapshot)
	} else {
		managedFactGap(ctx, "telemetry_invalid")
	}
}
func recordManagedCallTiming(ctx wrapper.HttpContext, s *managedExecutionState) {
	started, ok := ctx.GetContext(wrapper.OriginalRequestStartContext).(time.Time)
	if !ok || s == nil {
		return
	}
	duration := time.Since(started).Milliseconds()
	snapshot, err := executionfacts.FreezeRuntimeTelemetry(executionfacts.RuntimeTelemetry{ClientInfo: s.client, ProducerID: s.config.settings.ProducerID, Kind: "call", CallID: s.callID, RequestID: wrapper.PreserveOriginalRequestID(ctx), DurationMS: &duration, ObservedAt: managedNow()})
	if err == nil {
		dispatchManagedSnapshot(ctx, s, snapshot)
	}
}
func captureManagedUpstreamID(s *managedExecutionState, headers [][2]string) {
	for _, pair := range headers {
		name := strings.ToLower(pair[0])
		if name == "x-request-id" || name == "request-id" || name == "x-dashscope-request-id" {
			if len(pair[1]) > 0 && len(pair[1]) <= 256 && !strings.ContainsAny(pair[1], "\r\n") {
				s.upstreamRequestID = pair[1]
				return
			}
		}
	}
	if s.bailianRequestID != "" {
		s.upstreamRequestID = s.bailianRequestID
	}
}
func recordManagedAccessUpstream(ctx wrapper.HttpContext, s *managedExecutionState) {
	entry := managedAccessOrigin(ctx)
	if entry == nil {
		return
	}
	for index := range entry.Attempts {
		if entry.Attempts[index].AttemptID == s.attemptID {
			entry.Attempts[index].UpstreamRequestID = s.upstreamRequestID
			if !s.sendStartedAt.IsZero() {
				entry.Attempts[index].SentAt = s.sendStartedAt.UTC().Format(time.RFC3339Nano)
			}
		}
	}
	writeManagedAccessLog(ctx)
}
