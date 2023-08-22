go test -cpuprofile cpu.prof -memprofile mem.prof -bench .
go tool pprof cpu.prof
# go tool pprof mem.prof
