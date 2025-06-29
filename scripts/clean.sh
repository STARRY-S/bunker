#!/bin/bash
#!/bin/bash

cd $(dirname $0)/../

set -exuo pipefail

rm -r ./build/ || true
rm bunker || true

echo "Clean: Done"
