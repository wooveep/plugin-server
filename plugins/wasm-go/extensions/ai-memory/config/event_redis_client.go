package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/resp"
)

type EventRedisClient interface {
	Init() error
	Ready() bool
	Command(command []string, callback wrapper.RedisResponseCallback) error
}

type eventRedisClientImpl struct {
	clusterName     string
	initClusterName string
	username        string
	password        string
	timeout         uint32
	ready           bool
	authRetryAfter  time.Time
	recovering      bool
	generation      uint64
}

const eventRedisAuthRetryDelay = 30 * time.Second

var (
	eventRedisInit        = proxywasm.RedisInit
	eventRedisDispatch    = proxywasm.DispatchRedisCall
	eventRedisGetResponse = proxywasm.GetRedisCallResponse
	eventRedisNow         = time.Now
)

func newEventRedisClient(c RedisStreamConfig) *eventRedisClientImpl {
	clusterName := wrapper.FQDNCluster{
		FQDN: c.ServiceName,
		Port: int64(c.ServicePort),
	}.ClusterName()
	params := make([]string, 0, 3)
	if c.Database != 0 {
		params = append(params, fmt.Sprintf("db=%d", c.Database))
	}
	params = append(params, "buffer_flush_timeout=0", "max_buffer_size_before_flush=0")
	return &eventRedisClientImpl{
		clusterName:     clusterName,
		initClusterName: clusterName + "?" + strings.Join(params, "&"),
		username:        c.Username,
		password:        c.Password,
		timeout:         uint32(c.Timeout),
	}
}

func (c *eventRedisClientImpl) Init() error {
	if err := c.initialize(); err != nil {
		c.ready = false
		return nil
	}
	return nil
}

func (c *eventRedisClientImpl) Ready() bool {
	return c.ready
}

func (c *eventRedisClientImpl) Command(command []string, callback wrapper.RedisResponseCallback) error {
	if len(command) == 0 {
		return errors.New("memory event Redis command is empty")
	}
	reinitialized := false
	if !c.ready {
		if eventRedisNow().Before(c.authRetryAfter) {
			return errors.New("memory event Redis authentication retry is deferred")
		}
		c.recovering = false
		if err := c.initialize(); err != nil {
			c.deferAuthRetry()
			return err
		}
		reinitialized = true
	}
	return c.dispatch(eventRedisCommandQuery(command), callback, !reinitialized)
}

func (c *eventRedisClientImpl) initialize() error {
	if err := eventRedisInit(c.initClusterName, c.username, c.password, c.timeout); err != nil {
		c.ready = false
		return err
	}
	c.ready = true
	c.generation++
	return nil
}

func (c *eventRedisClientImpl) dispatch(query []byte, callback wrapper.RedisResponseCallback, retryAuth bool) error {
	generation := c.generation
	_, err := eventRedisDispatch(c.clusterName, query, func(status, responseSize int) {
		value := eventRedisResponseValue(status, responseSize)
		if eventRedisResponseRequiresAuth(value) {
			if generation != c.generation {
				if callback != nil {
					callback(value)
				}
				return
			}
			c.ready = false
			if retryAuth && !c.recovering && !eventRedisNow().Before(c.authRetryAfter) {
				c.recovering = true
				c.deferAuthRetry()
				if initErr := c.initialize(); initErr == nil {
					if retryErr := c.dispatch(query, callback, false); retryErr == nil {
						return
					}
				}
				c.ready = false
				c.recovering = false
			} else if !retryAuth {
				c.recovering = false
				c.deferAuthRetry()
			}
		} else if generation == c.generation {
			c.recovering = false
			c.authRetryAfter = time.Time{}
			if value.Error() == nil {
				c.ready = true
			}
		}
		if callback != nil {
			callback(value)
		}
	})
	return err
}

func (c *eventRedisClientImpl) deferAuthRetry() {
	c.authRetryAfter = eventRedisNow().Add(eventRedisAuthRetryDelay)
}

func eventRedisResponseValue(status, responseSize int) resp.Value {
	if status != 0 {
		return resp.ErrorValue(errors.New("memory event Redis call failed"))
	}
	response, err := eventRedisGetResponse(0, responseSize)
	if err != nil {
		return resp.ErrorValue(errors.New("memory event Redis response unavailable"))
	}
	reader := resp.NewReader(bytes.NewReader(response))
	value, _, err := reader.ReadValue()
	if err != nil && err != io.EOF {
		return resp.ErrorValue(errors.New("memory event Redis response invalid"))
	}
	return value
}

func eventRedisResponseRequiresAuth(value resp.Value) bool {
	err := value.Error()
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "NOAUTH")
}

func eventRedisCommandQuery(command []string) []byte {
	var buf bytes.Buffer
	writer := resp.NewWriter(&buf)
	values := make([]resp.Value, 0, len(command))
	for _, item := range command {
		values = append(values, resp.StringValue(item))
	}
	writer.WriteArray(values)
	return buf.Bytes()
}
