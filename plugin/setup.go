package dissident

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	"github.com/go-redis/redis"
	"github.com/mholt/caddy"
	"strconv"
)

func init() {
	caddy.RegisterPlugin("dissident", caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	d, err := dissidentParse(c)
	if err != nil {
		return plugin.Error("dissident", err)
	}

	// redis.NewClient returns a client to the Redis server specified by Options
	d.redis = redis.NewClient(&redis.Options{
		Addr:     d.redisAddr,
		Password: d.redisPassword,
		DB:       d.redisDB,
	})

	// PING the Redis server. Return an error if PONG was not received.
	_, err = d.redis.Ping().Result()
	if err != nil {
		return plugin.Error("dissident", err)
	}

	// Add a startup function that will -- after all plugins have been loaded -- check if the
	// prometheus plugin has been used - if so we will export metrics. We can only register
	// this metric once, hence the "once.Do".
	c.OnStartup(func() error {
		once.Do(func() {
			metrics.MustRegister(c, requestCount)
			metrics.MustRegister(c, allowedQueries)
			metrics.MustRegister(c, blockedQueries)
		})

		return nil
	})

	// Add the Plugin to CoreDNS, so Servers can use it in their plugin chain.
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		d.Next = next
		return d
	})

	return nil
}

func dissidentParse(c *caddy.Controller) (*Dissident, error) {
	// Default configuration
	d := Dissident{
		redisAddr:     "localhost:6379",
		redisPrefix:   "dissident",
		redisPassword: "",
		redisDB:       0,
	}

	c.Next()
	for c.NextBlock() {
		switch c.Val() {
		case "address":
			if !c.NextArg() {
				return &Dissident{}, c.ArgErr()
			}
			d.redisAddr = c.Val()
		case "prefix":
			if !c.NextArg() {
				return &Dissident{}, c.ArgErr()
			}
			d.redisPrefix = c.Val()
		case "password":
			if !c.NextArg() {
				return &Dissident{}, c.ArgErr()
			}
			d.redisPassword = c.Val()
		case "db":
			if !c.NextArg() {
				return &Dissident{}, c.ArgErr()
			}

			var err error

			d.redisDB, err = strconv.Atoi(c.Val())
			if err != nil {
				return &Dissident{}, err
			}
		default:
			if c.Val() != "}" {
				return &Dissident{}, c.Errf("unknown property '%s'", c.Val())
			}
		}
	}

	return &d, nil
}
