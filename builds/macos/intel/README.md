Tunneling is still WIP for MacOS, only exec will actually forward traffic
```bash
# start the client
./mercury start

# send a request through the client
./mercury exec curl icanhazip.com

# stop the client
./mercury stop
```