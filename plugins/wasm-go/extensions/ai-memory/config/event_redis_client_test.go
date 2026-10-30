package config

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/resp"
)

func TestEventRedisClientRecoversWithoutLoggingCommandPayload(t *testing.T) {
	originalInit := eventRedisInit
	originalDispatch := eventRedisDispatch
	originalGetResponse := eventRedisGetResponse
	originalNow := eventRedisNow
	t.Cleanup(func() {
		eventRedisInit = originalInit
		eventRedisDispatch = originalDispatch
		eventRedisGetResponse = originalGetResponse
		eventRedisNow = originalNow
	})

	t.Run("startup initialization failure recovers on the next command", func(t *testing.T) {
		initCalls := 0
		dispatchCalls := 0
		responseBody := []byte("$15\r\n1700000000000-0\r\n")
		eventRedisInit = func(cluster, username, password string, timeout uint32) error {
			initCalls++
			if initCalls == 1 {
				return errors.New("temporary init failure")
			}
			return nil
		}
		eventRedisDispatch = func(cluster string, query []byte, callback func(status, responseSize int)) (uint32, error) {
			dispatchCalls++
			callback(0, len(responseBody))
			return 1, nil
		}
		eventRedisGetResponse = func(start, maxSize int) ([]byte, error) {
			return responseBody, nil
		}

		client := newEventRedisClient(eventRedisClientTestConfig())
		require.NoError(t, client.Init())
		require.False(t, client.Ready())

		var delivered resp.Value
		require.NoError(t, client.Command(eventRedisClientTestCommand(), func(value resp.Value) {
			delivered = value
		}))
		require.Equal(t, 2, initCalls)
		require.Equal(t, 1, dispatchCalls)
		require.True(t, client.Ready())
		require.Equal(t, "1700000000000-0", delivered.String())
	})

	t.Run("NOAUTH reinitializes and retries once", func(t *testing.T) {
		initCalls := 0
		dispatchCalls := 0
		callbackCalls := 0
		responseBody := []byte("-NOAUTH Authentication required.\r\n")
		eventRedisInit = func(cluster, username, password string, timeout uint32) error {
			initCalls++
			return nil
		}
		eventRedisDispatch = func(cluster string, query []byte, callback func(status, responseSize int)) (uint32, error) {
			dispatchCalls++
			if dispatchCalls == 2 {
				responseBody = []byte("$15\r\n1700000000000-1\r\n")
			}
			callback(0, len(responseBody))
			return uint32(dispatchCalls), nil
		}
		eventRedisGetResponse = func(start, maxSize int) ([]byte, error) {
			return responseBody, nil
		}

		client := newEventRedisClient(eventRedisClientTestConfig())
		require.NoError(t, client.Init())
		require.NoError(t, client.Command(eventRedisClientTestCommand(), func(value resp.Value) {
			callbackCalls++
			require.Equal(t, "1700000000000-1", value.String())
		}))
		require.Equal(t, 2, initCalls)
		require.Equal(t, 2, dispatchCalls)
		require.Equal(t, 1, callbackCalls)
		require.True(t, client.Ready())
	})

	t.Run("persistent NOAUTH opens a retry cooldown", func(t *testing.T) {
		now := time.Unix(1_700_000_000, 0)
		initCalls := 0
		dispatchCalls := 0
		callbackCalls := 0
		responseBody := []byte("-NOAUTH Authentication required.\r\n")
		eventRedisNow = func() time.Time { return now }
		eventRedisInit = func(cluster, username, password string, timeout uint32) error {
			initCalls++
			return nil
		}
		eventRedisDispatch = func(cluster string, query []byte, callback func(status, responseSize int)) (uint32, error) {
			dispatchCalls++
			callback(0, len(responseBody))
			return uint32(dispatchCalls), nil
		}
		eventRedisGetResponse = func(start, maxSize int) ([]byte, error) {
			return responseBody, nil
		}

		client := newEventRedisClient(eventRedisClientTestConfig())
		require.NoError(t, client.Init())
		require.NoError(t, client.Command(eventRedisClientTestCommand(), func(value resp.Value) {
			callbackCalls++
		}))
		require.Equal(t, 2, initCalls)
		require.Equal(t, 2, dispatchCalls)
		require.Equal(t, 1, callbackCalls)
		require.False(t, client.Ready())

		require.Error(t, client.Command(eventRedisClientTestCommand(), nil))
		require.Equal(t, 2, initCalls, "cooldown must suppress per-event RedisInit")
		require.Equal(t, 2, dispatchCalls, "cooldown must suppress calls on the rejected client")

		now = now.Add(eventRedisAuthRetryDelay + time.Second)
		require.NoError(t, client.Command(eventRedisClientTestCommand(), func(value resp.Value) {
			callbackCalls++
		}))
		require.Equal(t, 3, initCalls, "one recovery initialization is allowed after cooldown")
		require.Equal(t, 3, dispatchCalls, "a post-cooldown NOAUTH must not trigger a second same-command retry")
		require.Equal(t, 2, callbackCalls)
		require.False(t, client.Ready())
	})

	t.Run("stale in-flight NOAUTH callbacks cannot start another recovery", func(t *testing.T) {
		now := time.Unix(1_700_000_000, 0)
		initCalls := 0
		var callbacks []func(status, responseSize int)
		responseBody := []byte("-NOAUTH Authentication required.\r\n")
		eventRedisNow = func() time.Time { return now }
		eventRedisInit = func(cluster, username, password string, timeout uint32) error {
			initCalls++
			return nil
		}
		eventRedisDispatch = func(cluster string, query []byte, callback func(status, responseSize int)) (uint32, error) {
			callbacks = append(callbacks, callback)
			return uint32(len(callbacks)), nil
		}
		eventRedisGetResponse = func(start, maxSize int) ([]byte, error) {
			return responseBody, nil
		}

		client := newEventRedisClient(eventRedisClientTestConfig())
		require.NoError(t, client.Init())
		firstCallbackCalls := 0
		secondCallbackCalls := 0
		require.NoError(t, client.Command(eventRedisClientTestCommand(), func(value resp.Value) {
			firstCallbackCalls++
			require.Equal(t, "1700000000000-2", value.String())
		}))
		require.NoError(t, client.Command(eventRedisClientTestCommand(), func(value resp.Value) {
			secondCallbackCalls++
			require.Error(t, value.Error())
		}))
		require.Len(t, callbacks, 2)

		callbacks[0](0, len(responseBody))
		require.Equal(t, 2, initCalls)
		require.Len(t, callbacks, 3, "the first NOAUTH may create one recovery dispatch")

		callbacks[1](0, len(responseBody))
		require.Equal(t, 2, initCalls, "the stale callback must not reinitialize the new generation")
		require.Len(t, callbacks, 3, "the stale callback must not dispatch another retry")
		require.Equal(t, 1, secondCallbackCalls)

		responseBody = []byte("$15\r\n1700000000000-2\r\n")
		callbacks[2](0, len(responseBody))
		require.Equal(t, 1, firstCallbackCalls)
		require.True(t, client.Ready())
	})
}

func TestSharedRedisOperationTimeout(t *testing.T) {
	tests := []struct {
		stream int
		recent int
		want   int
	}{
		{stream: 500, recent: 50, want: 50},
		{stream: 30, recent: 50, want: 30},
		{stream: 0, recent: 50, want: 50},
		{stream: 500, recent: 0, want: 500},
	}
	for _, tt := range tests {
		require.Equal(t, tt.want, sharedRedisOperationTimeout(tt.stream, tt.recent))
	}
}

func eventRedisClientTestConfig() RedisStreamConfig {
	return RedisStreamConfig{
		ServiceName: "redis.shared",
		ServicePort: 6379,
		Database:    2,
		Timeout:     500,
	}
}

func eventRedisClientTestCommand() []string {
	return []string{"XADD", "memory:events", "*", "event", "raw-payload-must-not-be-logged"}
}
