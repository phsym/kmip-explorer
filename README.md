# kmip-explorer

Browse and manage your KMIP objects in your terminal.
It supports KMIP from v1.0 up to v1.4.

![image](https://github.com/user-attachments/assets/1265c216-1c77-4816-8df6-3286a964ae2c)

# Quick start

## Installation

### From release
Download the latest release from the [release page](https://github.com/phsym/kmip-explorer/releases/latest)

### Install with go
Run `go install github.com/phsym/kmip-explorer@latest`

## Run it
Display help with `kmip-explorer -h`
```
Usage of kmip-explorer:
  -addr string
        Address and port of the KMIP Server
  -ca string
        Server's CA (optional)
  -cert string
        Path to the client certificate 
  -key string
        Path to the client private key
  -version
        Display version information
```

And run it with 
```bash
kmip-explorer -addr okms.gra.preprod.enablers.ovh:5696 -cert client.crt -key client.key
```