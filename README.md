# Mercury

## MacOS
Tunneling is still WIP for MacOS, only exec will actually forward traffic

### ARM
```bash
cd cd builds/macos/arm

# start the client
./mercury start

# send a request through the client
./mercury exec curl icanhazip.com

# stop the client
./mercury stop
```

### Intel
```bash
cd cd builds/macos/intel

# start the client
./mercury start

# send a request through the client
./mercury exec curl icanhazip.com

# stop the client
./mercury stop
```

## Linux
Mercury forwards all traffic on a system
```bash
# start the client
sudo ./mercury start

# stop the client
./mercury stop
```

## Common
```bash
# change the number of hops(defalt: 1, max: 2)
sudo ./mercury config circuit.hops 2
```

## Build from source code

### MacOS ARM
```bash
go build -o builds/macos/arm/mercury cmd/cli/main.go
```