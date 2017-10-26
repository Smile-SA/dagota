#!/bin/bash

cd /opt/app-root/src 


SEEDTOKEN=${DYN_TOKEN}$(hostname | sed 's/[^0-9]//g')
sed -i 's/%DYN_DC%/'${DYN_DC}'/g' /opt/app-root/src/conf/dynomite.yml
sed -i 's/%DYN_RACK%/'${DYN_RACK}'/g' /opt/app-root/src/conf/dynomite.yml
echo "  tokens: '${SEEDTOKEN}'" >> /opt/app-root/src/conf/dynomite.yml

cat /opt/app-root/src/conf/dynomite.yml

# set it up to use dagota, a replacment for florida
# in out context to not have nodejs dependencies
export DYNOMITE_FLORIDA_PORT=8080
export DYNOMITE_FLORIDA_IP="127.0.0.1"
export DYNOMITE_FLORIDA_REQUEST="GET / HTTP/1.0
Host: 127.0.0.1
User-Agent: HTMLGET 1.0

"
exec "$@"

