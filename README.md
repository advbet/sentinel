# sentinel

Redis Sentinel support for `github.com/gomodule/redigo` library. To use this library create Config struct and pass it to
`NewPool()` constructor. `NewPool()` can return an error if config is invalid. You can validate config by using
`ValidateConfig()` function. 

Create such struct and fill it with values:
```
type RedisConfig struct{
    // Name of the master instances set
	Master string
    // Slice of Sentinel instances addresses
	Sentinels []string
    // Struct of timeouts used for Sentinel connection
	SentinelTimeouts struct {
        // Time after which connection to sentinel will be timeouted
		Connect time.Duration
        // Time after which read request to sentinel will be timeouted
		Read    time.Duration
        // Time after which write request to sentinel will be timeouted
		Write   time.Duration
	}
    // Struct of timeouts used for Redis connections
	RedisTimeouts struct {
        // Time after which connection to redis instance will be timeouted
		Connect time.Duration
        // Time after which read request to redis instance will be timeouted
		Read    time.Duration
        // Time after which write request to redis instance will be timeouted
		Write   time.Duration
	}
}
```
Then pass this Config struct to `sentinel.NewPool()`:
```
pool, err := sentinel.NewPool(conf)
```
Note that all values are required to create sentinel pool.