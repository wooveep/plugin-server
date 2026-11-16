// Copyright (c) 2022 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wrapper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/tidwall/resp"
)

type RedisResponseCallback func(response resp.Value)

type RedisClient interface {
	Init(username, password string, timeout int64, opts ...optionFunc) error
	// return whether redis client is ready
	Ready() bool
	// with this function, you can call redis as if you are using redis-cli
	Command(cmds []interface{}, callback RedisResponseCallback) error
	Eval(script string, numkeys int, keys, args []interface{}, callback RedisResponseCallback) error

	// Key
	Del(key string, callback RedisResponseCallback) error
	Exists(key string, callback RedisResponseCallback) error
	Expire(key string, ttl int, callback RedisResponseCallback) error
	Persist(key string, callback RedisResponseCallback) error

	// String
	Get(key string, callback RedisResponseCallback) error
	Set(key string, value interface{}, callback RedisResponseCallback) error
	SetEx(key string, value interface{}, ttl int, callback RedisResponseCallback) error
	SetNX(key string, value interface{}, ttl int, callback RedisResponseCallback) error
	MGet(keys []string, callback RedisResponseCallback) error
	MSet(kvMap map[string]interface{}, callback RedisResponseCallback) error
	Incr(key string, callback RedisResponseCallback) error
	Decr(key string, callback RedisResponseCallback) error
	IncrBy(key string, delta int, callback RedisResponseCallback) error
	DecrBy(key string, delta int, callback RedisResponseCallback) error

	// List
	LLen(key string, callback RedisResponseCallback) error
	RPush(key string, vals []interface{}, callback RedisResponseCallback) error
	RPop(key string, callback RedisResponseCallback) error
	LPush(key string, vals []interface{}, callback RedisResponseCallback) error
	LPop(key string, callback RedisResponseCallback) error
	LIndex(key string, index int, callback RedisResponseCallback) error
	LRange(key string, start, stop int, callback RedisResponseCallback) error
	LRem(key string, count int, value interface{}, callback RedisResponseCallback) error
	LInsertBefore(key string, pivot, value interface{}, callback RedisResponseCallback) error
	LInsertAfter(key string, pivot, value interface{}, callback RedisResponseCallback) error

	// Hash
	HExists(key, field string, callback RedisResponseCallback) error
	HDel(key string, fields []string, callback RedisResponseCallback) error
	HLen(key string, callback RedisResponseCallback) error
	HGet(key, field string, callback RedisResponseCallback) error
	HSet(key, field string, value interface{}, callback RedisResponseCallback) error
	HMGet(key string, fields []string, callback RedisResponseCallback) error
	HMSet(key string, kvMap map[string]interface{}, callback RedisResponseCallback) error
	HKeys(key string, callback RedisResponseCallback) error
	HVals(key string, callback RedisResponseCallback) error
	HGetAll(key string, callback RedisResponseCallback) error
	HIncrBy(key, field string, delta int, callback RedisResponseCallback) error
	HIncrByFloat(key, field string, delta float64, callback RedisResponseCallback) error

	// Set
	SCard(key string, callback RedisResponseCallback) error
	SAdd(key string, value []interface{}, callback RedisResponseCallback) error
	SRem(key string, values []interface{}, callback RedisResponseCallback) error
	SIsMember(key string, value interface{}, callback RedisResponseCallback) error
	SMembers(key string, callback RedisResponseCallback) error
	SDiff(key1, key2 string, callback RedisResponseCallback) error
	SDiffStore(destination, key1, key2 string, callback RedisResponseCallback) error
	SInter(key1, key2 string, callback RedisResponseCallback) error
	SInterStore(destination, key1, key2 string, callback RedisResponseCallback) error
	SUnion(key1, key2 string, callback RedisResponseCallback) error
	SUnionStore(destination, key1, key2 string, callback RedisResponseCallback) error

	// Sorted Set
	ZCard(key string, callback RedisResponseCallback) error
	ZAdd(key string, msMap map[string]interface{}, callback RedisResponseCallback) error
	ZCount(key string, min interface{}, max interface{}, callback RedisResponseCallback) error
	ZIncrBy(key string, member string, delta interface{}, callback RedisResponseCallback) error
	ZScore(key, member string, callback RedisResponseCallback) error
	ZRank(key, member string, callback RedisResponseCallback) error
	ZRevRank(key, member string, callback RedisResponseCallback) error
	ZRem(key string, members []string, callback RedisResponseCallback) error
	ZRange(key string, start, stop int, callback RedisResponseCallback) error
	ZRevRange(key string, start, stop int, callback RedisResponseCallback) error
}

type RedisClusterClient[C Cluster] struct {
	cluster        C
	clusterName    string
	ready          bool
	checkReadyFunc func() error
	option         redisOption
}

type redisOption struct {
	dataBase                    int
	bufferFlushTimeout          int  // in milliseconds, default 3ms when not set
	maxBufferSizeBeforeFlush    int  // in bytes, default 1024 bytes when not set
	bufferFlushTimeoutSet       bool // flag to indicate if bufferFlushTimeout was explicitly set
	maxBufferSizeBeforeFlushSet bool // flag to indicate if maxBufferSizeBeforeFlush was explicitly set
}

type optionFunc func(*redisOption)

func WithDataBase(dataBase int) optionFunc {
	return func(o *redisOption) {
		o.dataBase = dataBase
	}
}

func WithBufferFlushTimeout(timeout time.Duration) optionFunc {
	return func(o *redisOption) {
		o.bufferFlushTimeout = int(timeout.Milliseconds())
		o.bufferFlushTimeoutSet = true
	}
}

func WithMaxBufferSizeBeforeFlush(size int) optionFunc {
	return func(o *redisOption) {
		o.maxBufferSizeBeforeFlush = size
		o.maxBufferSizeBeforeFlushSet = true
	}
}

// WithDisableBuffer is a convenience function that disables all buffering by setting
// both buffer_flush_timeout and max_buffer_size_before_flush to 0.
// This is useful for latency-sensitive scenarios where immediate query execution is required.
func WithDisableBuffer() optionFunc {
	return func(o *redisOption) {
		o.bufferFlushTimeout = 0
		o.bufferFlushTimeoutSet = true
		o.maxBufferSizeBeforeFlush = 0
		o.maxBufferSizeBeforeFlushSet = true
	}
}

func NewRedisClusterClient[C Cluster](cluster C) *RedisClusterClient[C] {
	return &RedisClusterClient[C]{
		cluster: cluster,
		checkReadyFunc: func() error {
			return errors.New("redis client is not ready, please call Init() first")
		},
	}
}

func RedisCall(cluster Cluster, respQuery []byte, callback RedisResponseCallback) error {
	return redisCallInternal(cluster.ClusterName(), respQuery, callback, nil, nil)
}

func redisCallInternal(clusterName string, respQuery []byte, callback RedisResponseCallback, readyPtr *bool, checkReadyFunc func() error) error {
	return redisCallWithDispatch(clusterName, respQuery, callback, readyPtr, checkReadyFunc, proxywasm.DispatchRedisCall)
}

type redisDispatchFunc func(string, []byte, func(int, int)) (uint32, error)

type redisCallTrace struct {
	requestID string
	attempt   int
}

func (trace redisCallTrace) diagnostic(stage, category, status string, started time.Time) string {
	return fmt.Sprintf("redis operation request_id=%s attempt=%d stage=%s category=%s status=%s elapsed_us=%d",
		trace.requestID, trace.attempt, stage, category, status, time.Since(started).Microseconds())
}

// Only public SDK error identities are classified. Error text can contain data.
func redisSDKErrorCategory(err error) string {
	switch {
	case errors.Is(err, types.ErrorStatusNotFound):
		return "not_found"
	case errors.Is(err, types.ErrorStatusBadArgument):
		return "bad_argument"
	case errors.Is(err, types.ErrorStatusEmpty):
		return "empty"
	case errors.Is(err, types.ErrorStatusCasMismatch):
		return "cas_mismatch"
	case errors.Is(err, types.ErrorInternalFailure):
		return "internal_failure"
	case errors.Is(err, types.ErrorUnimplemented):
		return "unimplemented"
	default:
		return "unknown"
	}
}

func redisCallWithDispatch(clusterName string, respQuery []byte, callback RedisResponseCallback, readyPtr *bool, checkReadyFunc func() error, dispatch redisDispatchFunc) error {
	return redisCallAttempt(clusterName, respQuery, callback, readyPtr, checkReadyFunc, dispatch, redisCallTrace{requestID: uuid.New().String()})
}

func redisCallAttempt(clusterName string, respQuery []byte, callback RedisResponseCallback, readyPtr *bool, checkReadyFunc func() error, dispatch redisDispatchFunc, trace redisCallTrace) error {
	started := time.Now()
	_, err := dispatch(
		clusterName,
		respQuery,
		func(status int, responseSize int) {
			var responseValue resp.Value
			hostStatus := strconv.Itoa(status)
			if status != 0 {
				// Redis callback statuses are not the SDK hostcall Status enum.
				// Without a callback enum/reason, a nonzero value cannot identify a timeout.
				proxywasm.LogCritical(trace.diagnostic("callback", "host_status_unknown", hostStatus, started))
				responseValue = resp.ErrorValue(fmt.Errorf("redis host call failed (status=%d)", status))
			} else {
				response, err := proxywasm.GetRedisCallResponse(0, responseSize)
				if err != nil {
					proxywasm.LogCritical(trace.diagnostic("response_read", redisSDKErrorCategory(err), hostStatus, started))
					responseValue = resp.ErrorValue(fmt.Errorf("cannot get redis response"))
				} else {
					rd := resp.NewReader(bytes.NewReader(response))
					value, _, err := rd.ReadValue()
					if err != nil && err != io.EOF {
						proxywasm.LogCritical(trace.diagnostic("response_parse", "invalid_resp", hostStatus, started))
						responseValue = resp.ErrorValue(fmt.Errorf("cannot read redis response"))
					} else {
						responseValue = value
						if value.Error() == nil {
							proxywasm.LogDebug(trace.diagnostic("response", "success", hostStatus, started))
						} else if strings.Contains(value.Error().Error(), "NOAUTH Authentication required") {
							proxywasm.LogWarn(trace.diagnostic("response", "noauth", hostStatus, started))
						} else {
							proxywasm.LogError(trace.diagnostic("response", "redis_error", hostStatus, started))
						}
					}
				}
			}

			// Keep the original one-time NOAUTH retry and callback ownership.
			shouldInvokeCallback := true
			if responseValue.Error() != nil && readyPtr != nil && checkReadyFunc != nil {
				if strings.Contains(responseValue.Error().Error(), "NOAUTH Authentication required") {
					*readyPtr = false
					authStarted := time.Now()
					if err := checkReadyFunc(); err == nil {
						proxywasm.LogInfo(trace.diagnostic("reauth", "success", "unavailable", authStarted))
						retryTrace := trace
						retryTrace.attempt++
						retryErr := redisCallAttempt(clusterName, respQuery, callback, nil, nil, dispatch, retryTrace)
						if retryErr == nil {
							shouldInvokeCallback = false
						}
					} else {
						proxywasm.LogError(trace.diagnostic("reauth", redisSDKErrorCategory(err), "unavailable", authStarted))
					}
				}
			}

			if shouldInvokeCallback && callback != nil {
				callback(responseValue)
			}
		})
	if err != nil {
		proxywasm.LogCritical(trace.diagnostic("dispatch", redisSDKErrorCategory(err), "unavailable", started))
	} else {
		proxywasm.LogDebug(trace.diagnostic("dispatch", "accepted", "unavailable", started))
	}
	return err
}

func respString(args []interface{}) []byte {
	var buf bytes.Buffer
	wr := resp.NewWriter(&buf)
	arr := make([]resp.Value, 0)
	for _, arg := range args {
		arr = append(arr, resp.StringValue(fmt.Sprint(arg)))
	}
	wr.WriteArray(arr)
	return buf.Bytes()
}

func (c *RedisClusterClient[C]) Ready() bool {
	return c.ready
}

func (c *RedisClusterClient[C]) Init(username, password string, timeout int64, opts ...optionFunc) error {
	// Apply user options
	for _, opt := range opts {
		opt(&c.option)
	}

	clusterName := c.cluster.ClusterName()

	// Build query parameters based on options
	params := make([]string, 0, 3)

	// Add database parameter if configured
	if c.option.dataBase != 0 {
		params = append(params, fmt.Sprintf("db=%d", c.option.dataBase))
	}

	// Add buffer_flush_timeout parameter
	// Default: 3ms if not explicitly set by user
	// If explicitly set to 0, use 0 (disable timeout-based flushing)
	bufferTimeout := c.option.bufferFlushTimeout
	if !c.option.bufferFlushTimeoutSet {
		bufferTimeout = 3 // default 3ms when not configured
	}
	params = append(params, fmt.Sprintf("buffer_flush_timeout=%d", bufferTimeout))

	// Add max_buffer_size_before_flush parameter
	// Default: 1024 bytes if not explicitly set by user
	// If explicitly set to 0, use 0 (disable buffering, send immediately)
	bufferSize := c.option.maxBufferSizeBeforeFlush
	if !c.option.maxBufferSizeBeforeFlushSet {
		bufferSize = 1024 // default 1024 bytes when not configured
	}
	params = append(params, fmt.Sprintf("max_buffer_size_before_flush=%d", bufferSize))

	// Append all parameters to cluster name
	if len(params) > 0 {
		clusterName = fmt.Sprintf("%s?%s", clusterName, strings.Join(params, "&"))
	}
	c.clusterName = clusterName

	// Always set checkReadyFunc to support re-authentication
	c.checkReadyFunc = func() error {
		if c.ready {
			return nil
		}
		started := time.Now()
		trace := redisCallTrace{requestID: uuid.New().String()}
		initErr := proxywasm.RedisInit(clusterName, username, password, uint32(timeout))
		if initErr != nil {
			proxywasm.LogError(trace.diagnostic("authentication", redisSDKErrorCategory(initErr), "unavailable", started))
			return initErr
		}
		c.ready = true
		proxywasm.LogInfo(trace.diagnostic("authentication", "success", "unavailable", started))
		return nil
	}

	started := time.Now()
	trace := redisCallTrace{requestID: uuid.New().String()}
	err := proxywasm.RedisInit(clusterName, username, password, uint32(timeout))
	if err != nil {
		proxywasm.LogWarn(trace.diagnostic("initialization", redisSDKErrorCategory(err), "unavailable", started))
		c.ready = false
		return nil
	}
	c.ready = true
	return nil
}

func (c *RedisClusterClient[C]) Command(cmds []interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	return redisCallInternal(c.dispatchClusterName(), respString(cmds), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) Eval(script string, numkeys int, keys, args []interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	params := make([]interface{}, 0)
	params = append(params, "eval")
	params = append(params, script)
	params = append(params, numkeys)
	params = append(params, keys...)
	params = append(params, args...)
	return redisCallInternal(c.dispatchClusterName(), respString(params), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) dispatchClusterName() string {
	// RedisInit accepts query parameters for DB and buffering options, but
	// DispatchRedisCall addresses the Envoy cluster by its raw name.
	return c.cluster.ClusterName()
}

// Key
func (c *RedisClusterClient[C]) Del(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "del")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) Exists(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "exists")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) Expire(key string, ttl int, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "expire")
	args = append(args, key)
	args = append(args, ttl)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) Persist(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "persist")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

// String
func (c *RedisClusterClient[C]) Get(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "get")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) Set(key string, value interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "set")
	args = append(args, key)
	args = append(args, value)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SetEx(key string, value interface{}, ttl int, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "set")
	args = append(args, key)
	args = append(args, value)
	args = append(args, "ex")
	args = append(args, ttl)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SetNX(key string, value interface{}, ttl int, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "set")
	args = append(args, key)
	args = append(args, value)
	args = append(args, "nx")
	if ttl > 0 {
		args = append(args, "ex")
		args = append(args, ttl)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) MGet(keys []string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "mget")
	for _, k := range keys {
		args = append(args, k)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) MSet(kvMap map[string]interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "mset")
	for k, v := range kvMap {
		args = append(args, k)
		args = append(args, v)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) Incr(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "incr")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) Decr(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "decr")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) IncrBy(key string, delta int, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "incrby")
	args = append(args, key)
	args = append(args, delta)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) DecrBy(key string, delta int, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "decrby")
	args = append(args, key)
	args = append(args, delta)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

// List
func (c *RedisClusterClient[C]) LLen(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "llen")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) RPush(key string, vals []interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "rpush")
	args = append(args, key)
	for _, val := range vals {
		args = append(args, val)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) RPop(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "rpop")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) LPush(key string, vals []interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "lpush")
	args = append(args, key)
	for _, val := range vals {
		args = append(args, val)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) LPop(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "lpop")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) LIndex(key string, index int, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "lindex")
	args = append(args, key)
	args = append(args, index)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) LRange(key string, start, stop int, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "lrange")
	args = append(args, key)
	args = append(args, start)
	args = append(args, stop)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) LRem(key string, count int, value interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "lrem")
	args = append(args, key)
	args = append(args, count)
	args = append(args, value)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) LInsertBefore(key string, pivot, value interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "linsert")
	args = append(args, key)
	args = append(args, "before")
	args = append(args, pivot)
	args = append(args, value)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) LInsertAfter(key string, pivot, value interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "linsert")
	args = append(args, key)
	args = append(args, "after")
	args = append(args, pivot)
	args = append(args, value)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

// Hash
func (c *RedisClusterClient[C]) HExists(key, field string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hexists")
	args = append(args, key)
	args = append(args, field)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HDel(key string, fields []string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hdel")
	args = append(args, key)
	for _, field := range fields {
		args = append(args, field)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HLen(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hlen")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HGet(key, field string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hget")
	args = append(args, key)
	args = append(args, field)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HSet(key, field string, value interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hset")
	args = append(args, key)
	args = append(args, field)
	args = append(args, value)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HMGet(key string, fields []string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hmget")
	args = append(args, key)
	for _, field := range fields {
		args = append(args, field)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HMSet(key string, kvMap map[string]interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hmset")
	args = append(args, key)
	for k, v := range kvMap {
		args = append(args, k)
		args = append(args, v)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HKeys(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hkeys")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HVals(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hvals")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HGetAll(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hgetall")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HIncrBy(key, field string, delta int, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hincrby")
	args = append(args, key)
	args = append(args, field)
	args = append(args, delta)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) HIncrByFloat(key, field string, delta float64, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "hincrbyfloat")
	args = append(args, key)
	args = append(args, field)
	args = append(args, delta)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

// Set
func (c *RedisClusterClient[C]) SCard(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "scard")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SAdd(key string, vals []interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "sadd")
	args = append(args, key)
	for _, val := range vals {
		args = append(args, val)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SRem(key string, vals []interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "srem")
	args = append(args, key)
	for _, val := range vals {
		args = append(args, val)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SIsMember(key string, value interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "sismember")
	args = append(args, key)
	args = append(args, value)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SMembers(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "smembers")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SDiff(key1, key2 string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "sdiff")
	args = append(args, key1)
	args = append(args, key2)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SDiffStore(destination, key1, key2 string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "sdiffstore")
	args = append(args, destination)
	args = append(args, key1)
	args = append(args, key2)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SInter(key1, key2 string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "sinter")
	args = append(args, key1)
	args = append(args, key2)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SInterStore(destination, key1, key2 string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "sinterstore")
	args = append(args, destination)
	args = append(args, key1)
	args = append(args, key2)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SUnion(key1, key2 string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "sunion")
	args = append(args, key1)
	args = append(args, key2)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) SUnionStore(destination, key1, key2 string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "sunionstore")
	args = append(args, destination)
	args = append(args, key1)
	args = append(args, key2)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

// ZSet
func (c *RedisClusterClient[C]) ZCard(key string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "zcard")
	args = append(args, key)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) ZAdd(key string, msMap map[string]interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "zadd")
	args = append(args, key)
	for m, s := range msMap {
		args = append(args, s)
		args = append(args, m)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) ZCount(key string, min interface{}, max interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "zcount")
	args = append(args, key)
	args = append(args, min)
	args = append(args, max)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) ZIncrBy(key string, member string, delta interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "zincrby")
	args = append(args, key)
	args = append(args, delta)
	args = append(args, member)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) ZScore(key, member string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "zscore")
	args = append(args, key)
	args = append(args, member)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) ZRank(key, member string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "zrank")
	args = append(args, key)
	args = append(args, member)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) ZRevRank(key, member string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "zrevrank")
	args = append(args, key)
	args = append(args, member)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) ZRem(key string, members []string, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "zrem")
	args = append(args, key)
	for _, m := range members {
		args = append(args, m)
	}
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) ZRange(key string, start, stop int, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "zrange")
	args = append(args, key)
	args = append(args, start)
	args = append(args, stop)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}

func (c *RedisClusterClient[C]) ZRevRange(key string, start, stop int, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	args := make([]interface{}, 0)
	args = append(args, "zrevrange")
	args = append(args, key)
	args = append(args, start)
	args = append(args, stop)
	return redisCallInternal(c.dispatchClusterName(), respString(args), callback, &c.ready, c.checkReadyFunc)
}
