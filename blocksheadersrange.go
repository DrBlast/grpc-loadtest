package main

// import some pandora stuff
// and stuff you need for your scenario
// and protobuf contracts for your grpc service

import (
	"context"
	"log"
	"time"

	"github.com/spf13/afero"
	pb "github.com/wavesplatform/gowaves/pkg/grpc/generated/waves/node/grpc"
	"github.com/yandex/pandora/cli"
	phttp "github.com/yandex/pandora/components/phttp/import"
	"github.com/yandex/pandora/core"
	"github.com/yandex/pandora/core/aggregator/netsample"
	coreimport "github.com/yandex/pandora/core/import"
	"github.com/yandex/pandora/core/register"
	"google.golang.org/grpc"
)

type Ammo struct {
	Tag        string
	FromHeight uint32 `json:"fromHeight"`
	ToHeight   uint32 `json:"toHeight"`
	//Param2 string
	//Param3 string
}

type Sample struct {
	URL              string
	ShootTimeSeconds float64
}

type GunConfig struct {
	Target string `validate:"required"` // Configuration will fail, without target defined
}

type Gun struct {
	// Configured on construction.
	client *grpc.ClientConn
	conf   GunConfig
	// Configured on Bind, before shooting
	aggr core.Aggregator // May be your custom Aggregator.
	core.GunDeps
}

func NewGun(conf GunConfig) *Gun {
	return &Gun{conf: conf}
}

func (g *Gun) Bind(aggr core.Aggregator, deps core.GunDeps) error {
	// create gRPC stub at gun initialization
	conn, err := grpc.Dial(
		g.conf.Target,
		grpc.WithInsecure(),
		grpc.WithTimeout(time.Second),
		grpc.WithUserAgent("load test, pandora custom shooter"))
	if err != nil {
		log.Fatalf("FATAL: %s", err)
	}
	g.client = conn
	g.aggr = aggr
	g.GunDeps = deps
	return nil
}

func (g *Gun) Shoot(ammo core.Ammo) {
	customAmmo := ammo.(*Ammo)
	g.shoot(customAmmo)
}

func (g *Gun) case1_method(client pb.BlocksApiClient, ammo *Ammo) int {
	code := 0

	var height uint32
	var toHeight uint32
	var includeTx bool
	height = ammo.FromHeight
	toHeight = ammo.ToHeight
	includeTx = false

	out, err := client.GetBlockRange(context.Background(), &pb.BlockRangeRequest{
		FromHeight: height, ToHeight: toHeight, IncludeTransactions: includeTx,
	})
	if err != nil {
		log.Printf("FATAL: %s", err)
		code = 500
	}
	// надо ли это?
	recv, err := out.Recv()
	if err != nil {
		log.Printf("FATAL: %s", err)
		code = 500
	}
	if recv != nil {
		code = 200
	}
	log.Printf("%+v", recv)
	return code
}

func (g *Gun) shoot(ammo *Ammo) {
	code := 0
	sample := netsample.Acquire(ammo.Tag)

	conn := g.client
	client := pb.NewBlocksApiClient(conn)
	//client := pb.NewTransactionsApiClient(conn)

	switch ammo.Tag {
	case "GRPC_BLOCKS_HEADERS_RANGE":
		code = g.case1_method(client, ammo)
	default:
		code = 404
	}

	defer func() {
		sample.SetProtoCode(code)
		g.aggr.Report(sample)
	}()
}

func main() {
	//debug.SetGCPercent(-1)
	// Standard imports.
	fs := afero.NewOsFs()
	coreimport.Import(fs)
	// May not be imported, if you don't need http guns and etc.
	phttp.Import(fs)

	// Custom imports. Integrate your custom types into configuration system.
	coreimport.RegisterCustomJSONProvider("custom_provider", func() core.Ammo { return &Ammo{} })

	register.Gun("blocks_headers_range_gun", NewGun, func() GunConfig {
		return GunConfig{
			Target: "default target",
		}
	})

	cli.Run()
}
