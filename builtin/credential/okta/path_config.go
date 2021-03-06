package okta

import (
	"fmt"
	"net/url"

	"time"

	"github.com/hashicorp/vault/logical"
	"github.com/hashicorp/vault/logical/framework"
	"github.com/sstarcher/go-okta"
)

func pathConfig(b *backend) *framework.Path {
	return &framework.Path{
		Pattern: `config`,
		Fields: map[string]*framework.FieldSchema{
			"organization": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Okta organization to authenticate against",
			},
			"token": &framework.FieldSchema{
				Type:        framework.TypeString,
				Description: "Okta admin API token",
			},
			"base_url": &framework.FieldSchema{
				Type: framework.TypeString,
				Description: `The API endpoint to use. Useful if you
are using Okta development accounts.`,
			},
			"ttl": &framework.FieldSchema{
				Type:        framework.TypeDurationSecond,
				Description: `Duration after which authentication will be expired`,
			},
			"max_ttl": &framework.FieldSchema{
				Type:        framework.TypeDurationSecond,
				Description: `Maximum duration after which authentication will be expired`,
			},
		},

		Callbacks: map[logical.Operation]framework.OperationFunc{
			logical.ReadOperation:   b.pathConfigRead,
			logical.CreateOperation: b.pathConfigWrite,
			logical.UpdateOperation: b.pathConfigWrite,
		},

		ExistenceCheck: b.pathConfigExistenceCheck,

		HelpSynopsis: pathConfigHelp,
	}
}

// Config returns the configuration for this backend.
func (b *backend) Config(s logical.Storage) (*ConfigEntry, error) {
	entry, err := s.Get("config")
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}

	var result ConfigEntry
	if entry != nil {
		if err := entry.DecodeJSON(&result); err != nil {
			return nil, err
		}
	}

	return &result, nil
}

func (b *backend) pathConfigRead(
	req *logical.Request, d *framework.FieldData) (*logical.Response, error) {

	cfg, err := b.Config(req.Storage)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		return nil, nil
	}

	resp := &logical.Response{
		Data: map[string]interface{}{
			"organization": cfg.Org,
			"base_url":     cfg.BaseURL,
			"ttl":          cfg.TTL,
			"max_ttl":      cfg.MaxTTL,
		},
	}

	return resp, nil
}

func (b *backend) pathConfigWrite(
	req *logical.Request, d *framework.FieldData) (*logical.Response, error) {
	cfg, err := b.Config(req.Storage)
	if err != nil {
		return nil, err
	}

	// Due to the existence check, entry will only be nil if it's a create
	// operation, so just create a new one
	if cfg == nil {
		cfg = &ConfigEntry{}
	}

	org, ok := d.GetOk("organization")
	if ok {
		cfg.Org = org.(string)
	} else if req.Operation == logical.CreateOperation {
		cfg.Org = d.Get("organization").(string)
	}

	token, ok := d.GetOk("token")
	if ok {
		cfg.Token = token.(string)
	} else if req.Operation == logical.CreateOperation {
		cfg.Token = d.Get("token").(string)
	}

	baseURL, ok := d.GetOk("base_url")
	if ok {
		baseURLString := baseURL.(string)
		if len(baseURLString) != 0 {
			_, err = url.Parse(baseURLString)
			if err != nil {
				return logical.ErrorResponse(fmt.Sprintf("Error parsing given base_url: %s", err)), nil
			}
			cfg.BaseURL = baseURLString
		}
	} else if req.Operation == logical.CreateOperation {
		cfg.BaseURL = d.Get("base_url").(string)
	}

	ttl, ok := d.GetOk("ttl")
	if ok {
		cfg.TTL = time.Duration(ttl.(int)) * time.Second
	} else if req.Operation == logical.CreateOperation {
		cfg.TTL = time.Duration(d.Get("ttl").(int)) * time.Second
	}

	maxTTL, ok := d.GetOk("max_ttl")
	if ok {
		cfg.MaxTTL = time.Duration(maxTTL.(int)) * time.Second
	} else if req.Operation == logical.CreateOperation {
		cfg.MaxTTL = time.Duration(d.Get("max_ttl").(int)) * time.Second
	}

	jsonCfg, err := logical.StorageEntryJSON("config", cfg)
	if err != nil {
		return nil, err
	}
	if err := req.Storage.Put(jsonCfg); err != nil {
		return nil, err
	}

	return nil, nil
}

func (b *backend) pathConfigExistenceCheck(
	req *logical.Request, d *framework.FieldData) (bool, error) {
	cfg, err := b.Config(req.Storage)
	if err != nil {
		return false, err
	}

	return cfg != nil, nil
}

// OktaClient creates a basic okta client connection
func (c *ConfigEntry) OktaClient() *okta.Client {
	client := okta.NewClient(c.Org)
	if c.BaseURL != "" {
		client.Url = c.BaseURL
	}

	if c.Token != "" {
		client.ApiToken = c.Token
	}

	return client
}

// ConfigEntry for Okta
type ConfigEntry struct {
	Org     string        `json:"organization"`
	Token   string        `json:"token"`
	BaseURL string        `json:"base_url"`
	TTL     time.Duration `json:"ttl"`
	MaxTTL  time.Duration `json:"max_ttl"`
}

const pathConfigHelp = `
This endpoint allows you to configure the Okta and its
configuration options.

The Okta organization are the characters at the front of the URL for Okta.
Example https://ORG.okta.com
`
