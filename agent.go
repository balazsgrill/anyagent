package main

import (
	"context"
	"log"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/anytype-heart/core/application"
	"github.com/anyproto/anytype-heart/core/block"
	bimport "github.com/anyproto/anytype-heart/core/block/import"
	"github.com/anyproto/anytype-heart/core/block/object/objectcreator"
	"github.com/anyproto/anytype-heart/core/block/process"
	"github.com/anyproto/anytype-heart/core/block/simple/text"
	"github.com/anyproto/anytype-heart/core/event"
	"github.com/anyproto/anytype-heart/pb"
	"github.com/anyproto/anytype-heart/pkg/lib/bundle"
	"github.com/anyproto/anytype-heart/pkg/lib/database"
	"github.com/anyproto/anytype-heart/pkg/lib/localstore/objectstore"
	"github.com/anyproto/anytype-heart/pkg/lib/pb/model"
	"github.com/anyproto/anytype-heart/space"
	"github.com/anyproto/anytype-heart/util/pbtypes"
	"github.com/balazsgrill/extractld"
	"github.com/gogo/protobuf/types"
)

type AgentConfig struct {
	Storagepath string
	Mnemonic    string
	Accountid   string
}

type agent struct {
	AgentConfig

	idle     *idledetector
	instance *application.Service

	spaceService         space.Service
	personalSpace        space.Space
	objectStoreService   objectstore.ObjectStore
	objectCreatorService objectcreator.Service
	blockService         *block.Service
	importer             bimport.Importer
}

func New(config AgentConfig) *agent {
	return &agent{
		AgentConfig: config,
	}
}

func (a *agent) Init() {
	a.idle = &idledetector{
		duration: time.Second,
	}
	a.instance = application.New()
}

func (a *agent) getOrCreate(source string) (string, bool, error) {
	result, _, err := a.objectStoreService.QueryObjectIDs(database.Query{
		Filters: []*model.BlockContentDataviewFilter{
			{
				RelationKey: bundle.RelationKeySource.String(),
				Condition:   model.BlockContentDataviewFilter_Equal,
				Value:       pbtypes.String(source),
			},
		},
	})
	if err != nil {
		return "", false, err
	}
	if len(result) == 0 {
		// Create
		id, _, err := a.objectCreatorService.CreateObject(context.Background(), a.personalSpace.Id(), objectcreator.CreateObjectRequest{
			ObjectTypeKey: bundle.TypeKeyNote,
			Details: &types.Struct{
				Fields: map[string]*types.Value{
					bundle.RelationKeyDescription.String(): pbtypes.String(source),
					text.DetailsKeyFieldName:               pbtypes.String(""),
				},
			},
		})
		return id, true, err
	} else {
		return result[0], false, nil
	}
}

func (a *agent) Stop() {
	a.instance.Stop()
}

func (a *agent) mailToBlock(email extractld.Mail) error {
	_, _, err := a.importer.Import(context.Background(), &pb.RpcObjectImportRequest{
		SpaceId:               a.personalSpace.Id(),
		UpdateExistingObjects: true,
		Type:                  model.Import_External,
		Mode:                  pb.RpcObjectImportRequest_IGNORE_ERRORS,
		NoProgress:            true,
		IsMigration:           false,
		IsNewSpace:            false,
		Snapshots: []*pb.RpcObjectImportRequestSnapshot{
			&pb.RpcObjectImportRequestSnapshot{},
		},
	}, model.ObjectOrigin_none, process.NewNoOp())
	return err
}

func (a *agent) Start() {
	a.instance.SetClientVersion("win32cli", "0.0.0")
	a.instance.SetEventSender(event.NewCallbackSender(
		func(event *pb.Event) {
			a.idle.NotIdle()
		}))
	log.Println("WalletRecover")
	err := a.instance.WalletRecover(&pb.RpcWalletRecoverRequest{
		RootPath: a.Storagepath,
		Mnemonic: a.Mnemonic,
	})
	if err != nil {
		panic(err)
	}
	log.Println("WalletCreateSession")
	token, err := a.instance.CreateSession(&pb.RpcWalletCreateSessionRequest{
		Mnemonic: a.Mnemonic,
	})
	if err != nil {
		panic(err)
	}
	log.Println(token)

	log.Println("AccountSelect")
	_, err = a.instance.AccountSelect(context.Background(), &pb.RpcAccountSelectRequest{
		Id:                      a.Accountid,
		RootPath:                a.Storagepath,
		DisableLocalNetworkSync: false,
	})
	if err != nil {
		panic(err)
	}
	// Wait sync to stabilize (no events for a second?)
	a.idle.Wait()

	a.spaceService = app.MustComponent[space.Service](a.instance.GetApp())
	a.personalSpace, _ = a.spaceService.GetPersonalSpace(context.Background())

	a.objectStoreService = app.MustComponent[objectstore.ObjectStore](a.instance.GetApp())

	a.blockService = app.MustComponent[*block.Service](a.instance.GetApp())
	a.objectCreatorService = app.MustComponent[objectcreator.Service](a.instance.GetApp())

	a.importer = app.MustComponent[bimport.Importer](a.instance.GetApp())
}
