package main

import (
	"flag"
	"fmt"
	"os"
	"pprof-study/pprofin"
	"pprof-study/pprofin/allocimpl"
	"pprof-study/pprofin/blockimpl"
	"pprof-study/pprofin/groutineimpl"
	"pprof-study/pprofin/profileimpl"
	"time"

	"demo-kartos/internal/conf"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	"github.com/go-kratos/kratos/v2/transport/http/pprof"

	hh "net/http"
	pp "net/http/pprof"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	id, _ = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

func newApp(logger log.Logger, hs *http.Server, gs *grpc.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			hs,
			gs,
		),
	)
}

func main() {
	flag.Parse()
	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	app, cleanup, err := wireApp(bc.Server, bc.Data, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	go func() { // 初始化测试内
		pprofs := []pprofin.Pprof{
			&allocimpl.PprofAlloc{Buf: make([][]byte, 0)}, // 内存优化
			&groutineimpl.PprofGoroutine{},                // 协程优化
			&profileimpl.PprofProfile{},                   // CPU优化
			&blockimpl.PproBlock{},                        // 锁阻塞优化
		}
		fmt.Printf("%s\n", "---start---")
		for {
			for _, p := range pprofs {
				p.DoPprof()
				time.Sleep(time.Second)
			}
		}
	}()
	server := http.NewServer(
		http.Address("0.0.0.0:8882"),
	)
	server.HandlePrefix("/", NewHandler())
	server.Handle("/debug/pprof/", pprof.NewHandler())
	server.Handle("/debug/pprof/cmdline", pprof.NewHandler())
	server.Handle("/debug/pprof/profile", pprof.NewHandler())
	server.Handle("/debug/pprof/symbol", pprof.NewHandler())
	server.Handle("/debug/pprof/trace", pprof.NewHandler())

	go kratos.New(kratos.Server(server)).Run()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}

// NewHandler new a pprof handler.
func NewHandler() hh.Handler {
	mux := hh.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pp.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pp.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pp.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pp.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pp.Trace)
	return mux
}
