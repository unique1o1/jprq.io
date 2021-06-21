package jprq

import "sync"

type Jprq struct {
	baseHost string
	tunnels  sync.Map
}

func New(baseHost string) Jprq {
	return Jprq{
		baseHost: baseHost,
		tunnels:  sync.Map{},
	}
}
