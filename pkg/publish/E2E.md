# Live publisher E2E runbook

The unit tests in `pkg/publish/{ipfs,arweave}` cover wire-format
correctness, signing, and parallel fan-out without network access. This
document captures the steps to also run the live, env-gated tests
against real nodes.

Both live tests are guarded by `t.Skip()` when the relevant environment
variables are absent, so CI without these credentials remains green.

---

## IPFS

The IPFS publisher targets any Kubo-compatible `/api/v0/add` endpoint:
local kubo, Pinata, web3.storage, IPFS Cluster, Infura.

### Local kubo (recommended for E2E)

```bash
# 1. install + run kubo
brew install ipfs && ipfs init && ipfs daemon &

# 2. point the test at it
export IPFS_API_URL=http://127.0.0.1:5001
export IPFS_GATEWAY_URL=http://127.0.0.1:8080

go test ./pkg/publish/ipfs -run TestKuboPublisher_E2E -v
# expected:
#   --- PASS: TestKuboPublisher_E2E (~1s)
#       live CID: bafy… (URL http://127.0.0.1:8080/ipfs/bafy…)
```

### Pinata

```bash
export IPFS_API_URL=https://api.pinata.cloud
export IPFS_AUTH_HEADER="Bearer <pinata-jwt>"
go test ./pkg/publish/ipfs -run TestKuboPublisher_E2E -v
```

---

## Arweave

The Arweave publisher self-signs format-2 transactions with an Arweave
JWK wallet. Two backends are supported by the same code path:

### arlocal (local in-memory chain — recommended for E2E)

```bash
# 1. spin up arlocal
npx arlocal 1984 &

# 2. mint a wallet + fund it (one-shot)
curl http://localhost:1984/mint/<addr>/100000000000000

# 3. point the test at it
export ARWEAVE_NODE_URL=http://localhost:1984
export ARWEAVE_WALLET_PATH=./wallet.json

go test ./pkg/publish/arweave -run TestPublish_E2E -v
# expected:
#   --- PASS: TestPublish_E2E (~2s)
#       live Arweave tx: <43-char id>
```

### Production (arweave.net)

Requires a funded Arweave wallet (~0.0001 AR per kilobyte). Same
invocation; just swap the node URL:

```bash
export ARWEAVE_NODE_URL=https://arweave.net
export ARWEAVE_WALLET_PATH=./funded-wallet.json
go test ./pkg/publish/arweave -run TestPublish_E2E -v
```

---

## Full pipeline E2E

The CLI exercises both publishers via the standard archival path:

```bash
go build -o pixe ./cmd/pixe

pixe convert ./README.md -o readme.pixe

# IPFS only
IPFS_API_URL=http://127.0.0.1:5001 \
  ./pixe publish readme.pixe --target ipfs

# IPFS + Arweave (parallel fan-out)
IPFS_API_URL=http://127.0.0.1:5001 \
ARWEAVE_NODE_URL=http://localhost:1984 \
ARWEAVE_WALLET_PATH=./wallet.json \
  ./pixe publish readme.pixe --target ipfs,arweave
```

Exit code is non-zero when any configured publisher fails; per-publisher
errors are reported in the JSON output without aborting the others.
