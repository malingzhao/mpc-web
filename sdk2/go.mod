module github.com/okx/threshold-lib/sdk2

go 1.19

require (
	github.com/decred/dcrd/dcrec/edwards/v2 v2.0.3
	github.com/okx/threshold-lib v0.0.0
)

require (
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/decred/dcrd/dcrec/secp256k1/v2 v2.0.1 // indirect
)

replace github.com/okx/threshold-lib => ../
