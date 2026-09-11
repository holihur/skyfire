package manager

import (
	"time"
)

// nonNil ensures JSON slice fields serialize as [] instead of null.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// InterfacePatch describes mutable fields of an interface used by the update
// endpoint. Nil means "leave unchanged".
type InterfacePatch struct {
	ListenPort *int     `json:"listenPort"`
	Addresses  []string `json:"addresses"`
	MTU        *int     `json:"mtu"`
	DNS        []string `json:"dns"`
	Up         *bool    `json:"up"`
}

// PeerInput describes a peer create/update request. The frontend always sends
// a complete form, so every field is overwritten on update.
type PeerInput struct {
	Name                string   `json:"name"`
	Address             string   `json:"address"`
	PublicKey           string   `json:"publicKey"`
	GenerateKeys        bool     `json:"generateKeys"`
	PresharedKey        string   `json:"presharedKey"`
	WithPreshared       *bool    `json:"withPreshared,omitempty"`
	AllowedIPs          []string `json:"allowedIPs"`
	ClientRoutes        []string `json:"clientRoutes"`
	DNS                 []string `json:"dns"`
	Endpoint            string   `json:"endpoint"`
	PersistentKeepalive int      `json:"persistentKeepalive"`
	Description         string   `json:"description"`
	Enabled             bool     `json:"enabled"`
}

// InterfaceView is an interface plus its live runtime state.
type InterfaceView struct {
	Name           string      `json:"name"`
	PublicKey      string      `json:"publicKey"`
	ListenPort     int         `json:"listenPort"`
	Addresses      []string    `json:"addresses"`
	MTU            int         `json:"mtu"`
	DNS            []string    `json:"dns"`
	Up             bool        `json:"up"`
	Running        bool        `json:"running"`
	DryRun         bool        `json:"dryRun"`
	TotalPeers     int         `json:"totalPeers"`
	ConnectedPeers int         `json:"connectedPeers"`
	TransferRx     uint64      `json:"transferRx"`
	TransferTx     uint64      `json:"transferTx"`
	Peers          []*PeerView `json:"peers"`
	CreatedAt      time.Time   `json:"createdAt"`
	UpdatedAt      time.Time   `json:"updatedAt"`
}

// PeerView is a peer plus its live runtime state.
type PeerView struct {
	Name                string    `json:"name"`
	PublicKey           string    `json:"publicKey"`
	PresharedKey        string    `json:"presharedKey"`
	ClientToken         string    `json:"clientToken"`
	Address             string    `json:"address"`
	AllowedIPs          []string  `json:"allowedIPs"`
	ClientRoutes        []string  `json:"clientRoutes"`
	DNS                 []string  `json:"dns"`
	Endpoint            string    `json:"endpoint"`
	PersistentKeepalive int       `json:"persistentKeepalive"`
	Description         string    `json:"description"`
	Enabled             bool      `json:"enabled"`
	Connected           bool      `json:"connected"`
	LatestHandshake     time.Time `json:"latestHandshake"`
	TransferRx          uint64    `json:"transferRx"`
	TransferTx          uint64    `json:"transferTx"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}
