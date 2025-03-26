package main

type MetricService interface {
	Get() (MetricInfo, error)
}

type MetricInfo interface {
	// MetricService is a struct that contains the total memory, free memory, and used memory percentage.
	String() (string, error)
}
