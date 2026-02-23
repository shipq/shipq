package kill

// servicePort groups a service name with its default listening ports.
type servicePort struct {
	name  string
	ports []int
}

// defaultPorts is the ordered list of well-known ports occupied by services
// that "shipq start <service>" can launch.  The worker is intentionally
// excluded because it connects outbound and holds no server port.
var defaultPorts = []servicePort{
	{name: "postgres", ports: []int{5432}},
	{name: "mysql", ports: []int{3306}},
	{name: "redis", ports: []int{6379}},
	{name: "minio", ports: []int{9000, 9001}},
	{name: "centrifugo", ports: []int{8000}},
	{name: "server", ports: []int{8080}},
}
