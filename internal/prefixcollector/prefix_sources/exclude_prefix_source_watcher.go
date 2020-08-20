package prefix_sources

type ExcludePrefixesSourceWatcher interface {
	ErrorChan() <-chan error
	ResultChan() <-chan []string
}
