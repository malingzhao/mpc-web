module go-client

go 1.21

replace github.com/okx/threshold-lib => /Users/malltony/mpc/threshold-lib

require (
	github.com/decred/dcrd/dcrec/secp256k1/v2 v2.0.1
	github.com/gorilla/websocket v1.5.0
	github.com/okx/threshold-lib v0.0.0
)

require (
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/decred/dcrd/dcrec/edwards/v2 v2.0.3 // indirect
)
