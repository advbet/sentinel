package sentinel

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
)

// Client is an instance of Redis Sentinel client. It supports concurrent
// querying for master and slave addresses.
type Client struct {
	conn       redis.Conn
	options    []redis.DialOption
	addrs      []string
	activeAddr int
	sync.Mutex
}

// Config is a configuration struct. It is used by applications using
// this library to pass Redis Sentinel cluster configuration.
type Config struct {
	Master           string
	Sentinels        []string
	SentinelTimeouts struct {
		Connect time.Duration
		Read    time.Duration
		Write   time.Duration
	}
	RedisTimeouts struct {
		Connect time.Duration
		Read    time.Duration
		Write   time.Duration
	}
}

// NewPool creates redigo/redis.Pool instance based on Config struct provided.
// Pool instance is safe to be used by redigo library. Error is returned if config is invalid
func NewPool(conf Config) (*redis.Pool, error) {
	if err := validateConfig(conf); err != nil {
		return nil, err
	}

	sentConn := NewClient(
		conf.Sentinels,
		redis.DialConnectTimeout(conf.SentinelTimeouts.Connect),
		redis.DialReadTimeout(conf.SentinelTimeouts.Read),
		redis.DialWriteTimeout(conf.SentinelTimeouts.Write),
	)

	sap := &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			masterAddr, err := sentConn.MasterAddress(conf.Master)
			if err != nil {
				return nil, fmt.Errorf("sentinel: get master address: %s", err)
			}
			c, err := redis.Dial(
				"tcp",
				masterAddr,
				redis.DialConnectTimeout(conf.RedisTimeouts.Connect),
				redis.DialReadTimeout(conf.RedisTimeouts.Read),
				redis.DialWriteTimeout(conf.RedisTimeouts.Write),
			)
			if err != nil {
				return nil, fmt.Errorf("dial error: %s", err)
			}
			if err := TestRole(c, "master"); err != nil {
				return nil, fmt.Errorf("dial: failed role check: %s", err)
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if err := TestRole(c, "master"); err != nil {
				return fmt.Errorf("failed role check: %s", err)
			}
			return nil
		},
	}

	return sap, nil
}

// NewClient creates a new sentinel client connection. Dial options passed to
// this function will be used when connecting to the sentinel server. Make sure
// to provide a short timeouts for all options (connect, read, write) as per
// redis-sentinel client guidelines.
//
// Note that in a worst-case scenario, the timeout for performing an
// operation with a Client client may take (# sentinels) * timeout to try all
// configured sentinel addresses.
func NewClient(addrs []string, options ...redis.DialOption) *Client {
	return &Client{
		options: options,
		addrs:   addrs,
	}
}

// do will atempt to execute single redis command on any of the configured
// sentinel servers. In worst case it will try all sentinel servers exactly once
// and return last encountered error.
func (sc *Client) do(cmd string, args ...interface{}) (interface{}, error) {
	var err error
	var reply interface{}

	for i := 0; i < len(sc.addrs); i++ {
		reply, err = sc.doOnce(cmd, args...)
		if err != nil {
			// Retry with the next sentinel in the list.
			sc.activeAddr = (sc.activeAddr + 1) % len(sc.addrs)
			continue
		}
		break
	}

	return reply, err
}

// doOnce tries to execute single redis command on the sentinel connection. If
// necessary it will dial before sending command.
func (sc *Client) doOnce(cmd string, args ...interface{}) (interface{}, error) {
	if sc.conn == nil {
		var err error
		sc.conn, err = redis.Dial("tcp", sc.addrs[sc.activeAddr], sc.options...)
		if err != nil {
			return nil, err
		}
	}

	reply, err := sc.conn.Do(cmd, args...)
	if err != nil {
		sc.conn.Close()
		sc.conn = nil
	}
	return reply, err
}

// MasterAddress looks up the configuration for a named monitored
// instance set and returns the master's configuration.
func (sc *Client) MasterAddress(name string) (string, error) {
	sc.Lock()
	defer sc.Unlock()

	res, err := redis.Strings(sc.do("SENTINEL", "get-master-addr-by-name", name))
	masterAddr := strings.Join(res, ":")
	return masterAddr, err
}

// Close will close connection to the sentinel server if one is esatablised.
func (sc *Client) Close() {
	sc.Lock()
	defer sc.Unlock()

	if sc.conn != nil {
		sc.conn.Close()
		sc.conn = nil
	}
}

// TestRole is a convenience function for checking redis server role. It
// uses the ROLE command introduced in redis 2.8.12. Nil is returned if server
// role matches the expected role.
//
// It is recommended by the redis client guidelines to test the role of any
// newly established connection before use.
func TestRole(c redis.Conn, expectedRole string) error {
	res, err := redis.Values(c.Do("ROLE"))
	if err != nil {
		return err
	}
	role, err := redis.String(res[0], nil)
	if err != nil {
		return err
	}
	if role != expectedRole {
		return errors.New("role check failed")
	}
	return nil
}

func validateConfig(conf Config) error {
	if conf.Master == "" {
		return errors.New("master is not set")
	}
	if len(conf.Sentinels) == 0 {
		return errors.New("sentinel array is not set")
	}
	if conf.SentinelTimeouts.Connect.Nanoseconds() == 0 ||
		conf.SentinelTimeouts.Read.Nanoseconds() == 0 ||
		conf.SentinelTimeouts.Write.Nanoseconds() == 0 {
		return errors.New("sentinel timeouts are not set")
	}
	if conf.RedisTimeouts.Connect.Nanoseconds() == 0 {
		return errors.New("redis timeouts are not set")
	}

	return nil
}
