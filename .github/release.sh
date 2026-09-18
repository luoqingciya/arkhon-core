#!/bin/bash

# =============================================================================
# 版本一致性硬校验：防止产物版本（$VERSION）与当前 git tag 漂移。
#   原理：constant.Version 的默认值仅是 -ldflags 注入前的占位，构建期真实版本
#        由 ldflags 注入（通常取自 git tag）；本校验以 git tag 作为权威对照。
#   - 处于 tag 上：必须与 $VERSION 完全一致，否则 exit 1 阻止发布。
#   - 非 tag（如 dev-/shortsha）：仅打印提示，不强校验。
#   可设 NO_VERSION_CHECK=1 跳过（例如本地手动整理产物时）。
# =============================================================================
if [ -z "${NO_VERSION_CHECK:-}" ]; then
    if git describe --tags --exact-match >/dev/null 2>&1; then
        TAG="$(git describe --tags --exact-match 2>/dev/null)"
        if [ "$VERSION" != "$TAG" ]; then
            echo "ERROR: version mismatch! VERSION='${VERSION}' != git tag '${TAG}'" >&2
            echo "refusing to release with a drifted version. aborting." >&2
            exit 1
        fi
        echo "version check ok: VERSION matches git tag '${TAG}'"
    else
        echo "warning: not on a git tag; skipping strict version/tag consistency check (VERSION=${VERSION})."
    fi
fi

FILENAMES=$(ls)
for FILENAME in $FILENAMES
do
    if [[ ! ($FILENAME =~ ".exe" || $FILENAME =~ ".sh")]];then
        gzip -S ".gz" $FILENAME
    elif [[ $FILENAME =~ ".exe" ]];then
        zip -m ${FILENAME%.*}.zip $FILENAME
    else echo "skip $FILENAME"
    fi
done

FILENAMES=$(ls)
for FILENAME in $FILENAMES
do
    if [[ $FILENAME =~ ".zip" ]];then
        echo "rename $FILENAME"
        mv $FILENAME ${FILENAME%.*}-${VERSION}.zip
    elif [[ $FILENAME =~ ".gz" ]];then
        echo "rename $FILENAME"
        mv $FILENAME ${FILENAME%.*}-${VERSION}.gz
    else
        echo "skip $FILENAME"
    fi
done