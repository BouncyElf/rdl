package rdl

import (
	"time"

	"github.com/garyburd/redigo/redis"
	goredis "github.com/go-redis/redis"
)

var cmdScript string = `
local v=redis.call("get", KEYS[1]);
if (v == nil) or (v ~= nil and v == ARGV[3]) then
    return redis.call("setex", KEYS[1], ARGV[1], ARGV[2])
end
return false
`

type RedigoClient struct {
	pool *redis.Pool
}

func NewRedigo(pool *redis.Pool) *RedigoClient {
	return &RedigoClient{
		pool: pool,
	}
}

func (cli *RedigoClient) SetIfValIs(
	k string,
	newVal string,
	ex time.Duration,
	origin string,
) (ok bool) {
	c := cli.pool.Get()
	defer c.Close()

	s := redis.NewScript(1, cmdScript)
	_, err := s.Do(c, k, newVal, ex.Seconds(), origin)
	return err == nil
}

type GoRedisClient struct {
	conn *goredis.Client
}

func NewGoRedis(conn *goredis.Client) *GoRedisClient {
	return &GoRedisClient{
		conn: conn,
	}
}

func (cli *GoRedisClient) SetIfValIs(
	k string,
	newVal string,
	ex time.Duration,
	origin string,
) (ok bool) {
	s := goredis.NewScript(cmdScript)
	_, err := s.Run(cli.conn, []string{k}, newVal, ex.Seconds(), origin).Result()
	return err == nil
}
