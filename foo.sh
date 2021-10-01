
GITHUB_ENV=foo.txt
IMG_BASE=$(grep '^IMAGE_TAG_BASE' -- Makefile | cut -f3 -d ' ')

RELEASE=rel
VERSION=version
CHANNEL=nightly
echo > $GITHUB_ENV
echo "RELEASE=$RELEASE" >> $GITHUB_ENV
echo "VERSION=$VERSION" >> $GITHUB_ENV
echo "CHANNEL=$CHANNEL" >> $GITHUB_ENV
echo "IMG=${IMG_BASE}:$VERSION" >> $GITHUB_ENV
echo "BUNDLE_IMG=${IMG_BASE}-bundle:$VERSION" >> $GITHUB_ENV
echo "CATALOG_IMG=${IMG_BASE}-catalog:latest" >> $GITHUB_ENV
echo "CATALOG_BASE_IMG=${IMG_BASE}-catalog:latest" >> $GITHUB_ENV
