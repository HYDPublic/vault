package database

import (
	"database/sql"
	"strings"
	"sync"
	"fmt"
	"encoding/json"

	log "github.com/mgutz/logxi/v1"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
)

func Factory(conf *logical.BackendConfig) (logical.Backend, error) {
	return Backend(conf).Setup(conf)
}

func Backend(conf *logical.BackendConfig) *backend {
	var b backend
	b.Backend = &framework.Backend{
		Help: strings.TrimSpace(backendHelp),

		Paths: []*framework.Path{
			pathConfigConnection(&b),
			pathListDBs(&b),
			pathConfigLease(&b),
			pathListRoles(&b),
			pathRoles(&b),
			pathRoleCreate(&b),
		},

		Secrets: []*framework.Secret{
			secretCreds(&b),
		},

		//Clean: ResetDB,
	}

	b.logger = conf.Logger
	b.dbs = make(map[string]*sql.DB)
	
	return &b
}

type backend struct {
	*framework.Backend

	lock sync.Mutex
	dbs  map[string]*sql.DB

	logger log.Logger
}

type Databases interface {
    Connect(*sql.DB) error
}

func (b *backend) DBConnection(s logical.Storage, name string) (*sql.DB, error) {
	b.logger.Trace("db: enter")
	defer b.logger.Trace("db: exit")

	b.lock.Lock()
	defer b.lock.Unlock()
	
	// Attempt to find connection configuration
	entry, err := s.Get("dbs/"+name)
	if err != nil {
		fmt.Println("can't find dbs/%s", name)
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("configure the DB connection with dbs/<name> first")
	}
	
	var dbInfo configDB
	if err := entry.DecodeJSON(&dbInfo); err != nil {
		return nil, err
	}
	
	switch dbInfo.DatabaseType {
	case "postgres":
		var config configPostgres
		err = json.Unmarshal(dbInfo.ConfigInfo, &config)
		if err != nil {
			return nil, fmt.Errorf("failure to unmarshal config data")
		}
		err = Connect(b.dbs[name], config)
	case "mysql":
		var config configMySQL
		err = json.Unmarshal(dbInfo.ConfigInfo, &config)
		if err != nil {
			return nil, fmt.Errorf("failure to unmarshal config data")
		}
		err = Connect(b.dbs[name], config)
	default:
		return nil, fmt.Errorf("database type not recognized")
	}
	
	return b.dbs[name], nil
}

// ResetDB forces a connection on the next call to DBConnection()
func (b *backend) ResetDB(name string) {
	b.logger.Trace("db/resetdb: enter")
	defer b.logger.Trace("db/resetdb: exit")

	b.lock.Lock()
	defer b.lock.Unlock()

	if b.dbs[name] != nil {
		b.dbs[name].Close()
	}

	b.dbs[name] = nil
}


func (b *backend) Lease(s logical.Storage) (*configLease, error) {
	entry, err := s.Get("config/lease")
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	var result configLease
	if err := entry.DecodeJSON(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

const backendHelp = `
The database backend dynamically generates database users.

After mounting this backend, configure it using the endpoints within
the "dbs/" path.
`
