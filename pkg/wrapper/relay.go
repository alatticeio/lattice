package wrapper

type Relayer struct {
	bind *LinkBind
}

func NewRelayer(bind *LinkBind) *Relayer {
	return &Relayer{
		bind: bind,
	}
}
