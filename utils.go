package main

import (
	"crypto/sha256"
	"fmt"

	"github.com/gliderlabs/ssh"
	"github.com/gomodule/redigo/redis"
)

func (s *Server) getConn() (redis.Conn, error) {
	return redis.DialURL(s.cfg.RedisURL)
}

func (s *Server) getID(key ssh.PublicKey) string {
	h := sha256.New()
	h.Write(key.Marshal())
	return fmt.Sprintf("%x", h.Sum(nil))
}
