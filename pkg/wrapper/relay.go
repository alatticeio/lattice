package wrapper

type Relayer struct {
	bind *NetBind
}

func NewRelayer(bind *NetBind) *Relayer {
	return &Relayer{
		bind: bind,
	}
}
