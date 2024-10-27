package dissident

import (
	"context"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/metrics"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/request"
	"github.com/go-redis/redis"
	"github.com/miekg/dns"
	"github.com/satori/go.uuid"
	"strings"
	"time"
)

const Week = time.Hour * 24 * 7

var log = clog.NewWithPlugin("dissident")

type Dissident struct {
	Next          plugin.Handler
	redisAddr     string
	redisPassword string
	redisPrefix   string
	redisDB       int
	redis         *redis.Client
}

func (d Dissident) isBlocked(clientId, name string) bool {
	labels := strings.Split(name, ".")
	keys := make([]string, 0)

	for i, _ := range labels {
		key := d.redisPrefix + "/" + clientId + "/." + strings.Join(labels[i:len(labels)], ".")
		keys = append(keys, key)
	}

	keys = append(keys, d.redisPrefix+"/"+clientId+"/"+name)

	values, err := d.redis.MGet(keys...).Result()
	if err != nil {
		log.Error(err)
		return true
	}

	for i, value := range values {
		if value != nil {
			expires, err := time.ParseDuration(value.(string) + "s")
			if err != nil {
				log.Error(err)
				return true
			}

			ttl, err := d.redis.TTL(keys[i]).Result()
			if err != nil {
				log.Error(err)
				return true
			}

			delta := expires.Seconds() - ttl.Seconds()

			if delta <= expires.Seconds()/10 && delta > 600 && expires.Seconds() < 2764800 {
				_, err := d.redis.Set(keys[i], expires.Seconds()*2, expires*2).Result()
				if err != nil {
					log.Error(err)
					return true
				}
			} else {
				_, err := d.redis.Expire(keys[i], expires).Result()
				if err != nil {
					log.Error(err)
					return true
				}
			}

			return false
		}
	}

	return true
}

func (d Dissident) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	requestCount.WithLabelValues(metrics.WithServer(ctx)).Inc()

	req := request.Request{W: w, Req: r, Context: ctx}
	if req.QType() == dns.TypeA || req.QType() == dns.TypeAAAA {
		name := req.Name()
		if strings.HasSuffix(name, ".") {
			name = name[:len(name)-1]
		}

		key := d.redisPrefix + "/ip/" + req.IP()

		clientId, err := d.redis.Get(key).Result()
		if err != nil {
			if err != redis.Nil {
				log.Error(err)
			}

			c, err := uuid.NewV4()
			if err != nil {
				log.Error(err)
			}

			clientId = strings.Split(c.String(), "-")[4]
			_, err = d.redis.Set(key, clientId, Week).Result()
			if err != nil {
				log.Error(err)
			}
		} else {
			_, err := d.redis.Expire(key, Week).Result()
			if err != nil {
				log.Error(err)
			}
		}

		if d.isBlocked(clientId, name) {
			_, err = d.redis.Publish(d.redisPrefix+"/"+clientId, name).Result()
			if err != nil {
				log.Error(err)
			}

			msg := new(dns.Msg)
			msg.SetReply(r)
			msg.Authoritative, msg.RecursionAvailable = true, true
			msg.Rcode = dns.RcodeNameError
			w.WriteMsg(msg)

			blockedQueries.WithLabelValues(metrics.WithServer(ctx)).Inc()

			return msg.Rcode, nil
		} else {
			allowedQueries.WithLabelValues(metrics.WithServer(ctx)).Inc()
		}
	}

	return plugin.NextOrFailure(d.Name(), d.Next, ctx, w, r)
}

func (d Dissident) Name() string {
	return "dissident"
}
