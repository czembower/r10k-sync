// r10k-sync
// a foreground handler for distributed puppet masters running inside of kubernetes
// a CI pipeline should be used to push messages to a redis queue
// handler listens for messages from configured redis cluster that contain the changed puppet environment name as a string
// deploys code from the configured repository to the puppet environment using r10k

package main

import (
  "context"
  "fmt"
  "os"
  "os/exec"
  "time"

  "github.com/go-redis/redis/v8"
)

func main() {

  println("puppet-redis starting")

  var ctx = context.Background()

  redisClient := redis.NewFailoverClusterClient(&redis.FailoverOptions{
    MasterName:    "mymaster",
    SentinelAddrs: []string{"primary-redis.redis.svc.cluster.local:26379"},
    RouteRandomly: true,
  })

  subClient := redisClient.Subscribe(ctx, "puppet")
  channel := subClient.Channel()

  err := subClient.Ping(ctx)

  if err != nil {
    println("error initializing")
  } else {
    println("puppet-redis initialized")
  }

  println("executing initial r10k pull/deploy...")
  cmd := exec.Command("/usr/bin/r10k", "deploy", "environment", "--puppetfile", "-v", "--config=/etc/puppetlabs/puppet/r10k_code.yaml")
  out, err := cmd.CombinedOutput()

  if err != nil {
    fmt.Printf("%s \n", err)
    fmt.Printf("%s \n", out)
    os.Remove("/root/.puppet-redis-ok")
    println("error deploying with r10k")
  } else {
    println(string(out))
    os.Create("/root/.puppet-redis-ok")
    println("stabilized, waiting for messages...")
  }

  for msg := range channel {
    nowTime := time.Now().String()
    env := msg.Payload
    println(nowTime, "deploying environment:", env)
    cmd := exec.Command("/usr/bin/r10k", "deploy", "environment", env, "--puppetfile", "-v", "--config=/etc/puppetlabs/puppet/r10k_code.yaml")
    out, err := cmd.CombinedOutput()

    if err != nil {
      println("error deploying with r10k")
      os.Remove("/root/.puppet-redis-ok")
    } else {
      println(string(out))
      os.Create("/root/.puppet-redis-ok")
    }
  }
}
