package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/gliderlabs/ssh"
	"github.com/sirupsen/logrus"
	gossh "golang.org/x/crypto/ssh"
)

type ServerConfig struct {
	ListenPort  int
	KeysPath    string
	HostKeyPath string
}

type Server struct {
	cfg        *ServerConfig
	publicKeys []ssh.PublicKey
	mu         *sync.Mutex
}

func NewServer(cfg *ServerConfig) (*Server, error) {
	if cfg.ListenPort == 0 {
		cfg.ListenPort = 2222
	}
	return &Server{
		cfg: cfg,
		mu:  &sync.Mutex{},
	}, nil
}

func (s *Server) loadKeys() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cfg.KeysPath == "" {
		return nil
	}

	keys, err := ioutil.ReadDir(s.cfg.KeysPath)
	if err != nil {
		return err
	}

	pubKeys := []ssh.PublicKey{}
	for _, k := range keys {
		logrus.Debugf("loading public key %s", k.Name())
		p := filepath.Join(s.cfg.KeysPath, k.Name())
		data, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}
		k, _, _, _, err := ssh.ParseAuthorizedKey(data)
		if err != nil {
			return err
		}
		pubKeys = append(pubKeys, k)
	}

	s.publicKeys = pubKeys
	return nil
}

func (s *Server) Run() error {
	if err := s.loadKeys(); err != nil {
		return err
	}

	ssh.Handle(func(s ssh.Session) {
		authorizedKey := gossh.MarshalAuthorizedKey(s.PublicKey())
		io.WriteString(s, fmt.Sprintf("pub key used by %s\n", s.User()))
		s.Write(authorizedKey)
	})

	pubKeyOption := ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
		return s.isAuthorized(ctx, key)
	})

	logrus.Infof("starting ssh server on port %d", s.cfg.ListenPort)
	opts := []ssh.Option{
		pubKeyOption,
	}
	if _, err := os.Stat(s.cfg.HostKeyPath); err == nil {
		opts = append(opts, ssh.HostKeyFile(s.cfg.HostKeyPath))
	}
	return ssh.ListenAndServe(fmt.Sprintf(":%d", s.cfg.ListenPort), nil, pubKeyOption)
}

func (s *Server) isAuthorized(ctx ssh.Context, key ssh.PublicKey) bool {
	for _, k := range s.publicKeys {
		if ssh.KeysEqual(key, k) {
			return true
		}
	}
	logrus.WithFields(logrus.Fields{
		"user": ctx.User(),
		"addr": ctx.RemoteAddr(),
	}).Warn("access denied")
	return false
}
