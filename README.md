# kmip-explorer

Browse and manage your KMIP objects in your terminal.
It supports KMIP from v1.0 up to v1.4.

![image](https://github.com/user-attachments/assets/1265c216-1c77-4816-8df6-3286a964ae2c)

## Quick start

### Installation

#### From release
Download the latest release from the [release page](https://github.com/phsym/kmip-explorer/releases/latest)

#### Install with go
Run `go install github.com/phsym/kmip-explorer@latest`

### Run it
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
  -no-ccv
        Do not add client correlation value to requests
  -no-check-update
        Do not check for update
  -version
        Display version information
```

And run it with 
```bash
kmip-explorer -addr eu-west-rbx.okms.ovh.net:5696 -cert client.crt -key client.key
```

## Demo
[![asciicast](https://asciinema.org/a/CtasVyDZNQqVLwKvL5ej96ftR.svg)](https://asciinema.org/a/CtasVyDZNQqVLwKvL5ej96ftR)

## Compatibility
This project is developed using using [OVHcloud's KMIP client](https://github.com/ovh/kmip-go) and inherits the library compatibility. Checkout the [compatibility matrix](https://github.com/ovh/kmip-go/blob/main/README.md#implementation-status).

It supports KMIP protocol from v1.0 up to v1.4, and is tested against [OVHcloud's KMS service](https://help.ovhcloud.com/csm/en-ie-kms-quick-start?id=kb_article_view&sysparm_article=KB0063362).