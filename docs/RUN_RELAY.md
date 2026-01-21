# Run Relay Server

## Steps

### 1. Clone the repo

```bash
git clone https://github.com/unblink/unblink.git
cd unblink
```

### 2. Apply patches

```bash
go mod vendor
cd vendor/github.com/AlexxIT/go2rtc
patch -p1 < ../../../../patches/go2rtc-setconn.patch
cd ../../../..
```

### 3. Build

```bash
go build -mod=vendor -o relay ./cmd/relay
```

### 4. Run

```bash
./relay
```
