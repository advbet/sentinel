# sentinel

Redis Sentinel support for `github.com/gomodule/redigo` library. To use this library create Config struct and pass it to
`NewPool()` constructor. `NewPool()` can return an error if config is invalid. You can validate config by using
`ValidateConfig()` function. 

Create such struct and fill it with values:
```
type RedisConfig struct{
	Master string
	Sentinels []string
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
```
Then pass this Config struct to `sentinel.NewPool()`:
```
pool, err := sentinel.NewPool(conf)
```
Note that all values are required to create sentinel pool.