package utils

import "sync"

type SynchronizedPrefixList interface {
	Append(string)
	SetList([]string)
	GetList() []string
}

type SynchronizedPrefixListImpl struct {
	lock     sync.Mutex
	prefixes []string
}

func NewSynchronizedPrefixListImpl() *SynchronizedPrefixListImpl {
	return &SynchronizedPrefixListImpl{
		prefixes: make([]string, 0, 16),
	}
}

func (s *SynchronizedPrefixListImpl) Append(prefix string) {
	s.lock.Lock()
	s.prefixes = append(s.prefixes, prefix)
	s.lock.Unlock()
}

func (s *SynchronizedPrefixListImpl) GetList() []string {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.prefixes
}

func (s *SynchronizedPrefixListImpl) SetList(list []string) {
	s.lock.Lock()
	s.prefixes = list
	s.lock.Unlock()
}
