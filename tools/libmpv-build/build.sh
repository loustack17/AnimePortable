#!/usr/bin/env bash
# SPDX-License-Identifier: MPL-2.0
set -euo pipefail

: "${FFBUILD_PREFIX:?}"
: "${FFBUILD_TARGET_FLAGS:?}"
: "${FF_CONFIGURE:?}"
: "${FFBUILD_CROSS_PREFIX:?}"
: "${VARIANT:?}"

[[ "$VARIANT" == lgpl* ]]
[[ -d ffmpeg && -d mpv ]]

read -r -a target_flags <<< "$FFBUILD_TARGET_FLAGS"
read -r -a configure_flags <<< "$FF_CONFIGURE"

pushd ffmpeg
./configure \
    --prefix="$FFBUILD_PREFIX" \
    --pkg-config-flags=--static \
    "${target_flags[@]}" \
    "${configure_flags[@]}" \
    --extra-cflags="$FF_CFLAGS" \
    --extra-cxxflags="$FF_CXXFLAGS" \
    --extra-libs="$FF_LIBS" \
    --extra-ldflags="$FF_LDFLAGS" \
    --extra-ldexeflags="$FF_LDEXEFLAGS" \
    --cc="$CC" --cxx="$CXX" --ar="$AR" --ranlib="$RANLIB" --nm="$NM" \
    --enable-static --disable-shared --disable-doc --disable-programs \
    --disable-gpl --disable-nonfree
if grep -Eq '^CONFIG_(GPL|NONFREE)=yes$' ffbuild/config.mak; then
    echo 'GPL or nonfree FFmpeg configuration rejected' >&2
    exit 1
fi
make -j4
make install
popd

pushd mpv
mkdir -p ../evidence
printf '#include <d3d11sdklayers.h>\nconst GUID *dxgi_debug_id = &DXGI_DEBUG_D3D11;\n' | "$CC" -x c -c -o /dev/null -
meson setup build . \
    --cross-file /cross.meson \
    --wrap-mode=nodownload \
    --auto-features=disabled \
    --buildtype=release \
    --prefer-static \
    --default-library=shared \
    -Dc_args=-DHAVE_DXGI_DEBUG_D3D11=1 \
    -Dc_link_args="$FF_LIBS" \
    -Dcpp_link_args="$FF_LIBS" \
    -Dgpl=false \
    -Dcplayer=false \
    -Dlibmpv=true \
    -Dgl=enabled \
    -Dplain-gl=enabled \
    -Dd3d11=disabled \
    -Dd3d-hwaccel=enabled \
    -Dwasapi=enabled \
    -Dwin32-threads=enabled \
    -Dlua=disabled \
    -Djavascript=disabled \
    -Dsubrandr=disabled \
    -Dshaderc=disabled \
    -Dspirv-cross=disabled \
    -Dvulkan=disabled \
    -Dwin32-smtc=disabled
meson compile -C build -j4
cp build/libmpv-2.dll ../evidence/libmpv-2.dll
cp build/meson-info/intro-dependencies.json ../evidence/mpv-dependencies.json
cp build/meson-logs/meson-log.txt ../evidence/mpv-meson-log.txt
"${FFBUILD_CROSS_PREFIX}objdump" -p ../evidence/libmpv-2.dll > ../evidence/libmpv-pe.txt
sha256sum ../evidence/libmpv-2.dll > ../evidence/libmpv-sha256.txt
popd

cp ffmpeg/ffbuild/config.mak evidence/ffmpeg-config.mak
cp ffmpeg/ffbuild/config.log evidence/ffmpeg-config.log
