rancher-cube-apiserver
========

API server for RancherCUBE.

## Prepare

Generate JWT RS256 key to `/etc/rancher/cube`

```
mkdir -p /etc/rancher/cube && cd /etc/rancher/cube

ssh-keygen -t rsa -b 4096 -f id_rsa

# Don't add passphrase

openssl rsa -in id_rsa -pubout -outform PEM -out id_rsa.pub
```

## Building

`make`

## Running

`./bin/rancher-cube-apiserver`

## License
Copyright (c) 2018 [Rancher Labs, Inc.](http://rancher.com)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

[http://www.apache.org/licenses/LICENSE-2.0](http://www.apache.org/licenses/LICENSE-2.0)

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
