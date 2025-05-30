#!/bin/bash
# Example: Setup a provider with DIDs

PROVIDER_NAME=$1
PROVIDER_HOST=$2
PROVIDER_USER=$3
PROVIDER_PASS=$4

if [ -z "$PROVIDER_NAME" ]; then
   echo "Usage: $0 <provider_name> <host> [username] [password]"
   exit 1
fi

# Add provider
echo "Adding provider $PROVIDER_NAME..."
./bin/router provider add \
   --name "$PROVIDER_NAME" \
   --host "$PROVIDER_HOST" \
   --username "$PROVIDER_USER" \
   --password "$PROVIDER_PASS" \
   --codecs ulaw,alaw,g729 \
   --max-channels 200 \
   --country US

# Add some example DIDs
echo "Adding DIDs..."
./bin/router did add \
   --provider "$PROVIDER_NAME" \
   --dids "12125551000,12125551001,12125551002,12125551003,12125551004" \
   --country US

echo "Provider $PROVIDER_NAME configured!"
