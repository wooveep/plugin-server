package wrapper

// RedisEventClient keeps Command callbacks alive in the plugin context after
// the originating HTTP request ends. It is for event writes; callbacks must not
// access or resume the request. Other Redis methods keep request ownership.
type RedisEventClient[C Cluster] struct {
	*RedisClusterClient[C]
	dispatch redisDispatchFunc
}

// NewRedisEventClient takes the plugin's background SDK dispatcher explicitly;
// the ordinary wrapper client does not change its SDK version or ownership.
func NewRedisEventClient[C Cluster](cluster C, dispatch func(string, []byte, func(int, int)) (uint32, error)) *RedisEventClient[C] {
	return &RedisEventClient[C]{RedisClusterClient: NewRedisClusterClient(cluster), dispatch: dispatch}
}

func (c *RedisEventClient[C]) Command(cmds []interface{}, callback RedisResponseCallback) error {
	if err := c.checkReadyFunc(); err != nil {
		return err
	}
	return redisCallWithDispatch(c.dispatchClusterName(), respString(cmds), callback, &c.ready, c.checkReadyFunc, c.dispatch)
}
