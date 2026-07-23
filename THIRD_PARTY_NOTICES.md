# Third-Party Notices

## Fuwari

The frontend visual system is adapted from ideas and selected styling patterns in [saicaca/fuwari](https://github.com/saicaca/fuwari), inspected at commit `6d39b0dec41282e7852e23e032998a5789abee28`.

```text
MIT License

Copyright (c) 2024 saicaca

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## Artalk

The frontend uses the Artalk npm package and the deployment design uses the official Artalk Go image. The deployed image is pinned to the official nightly build from commit `75a35ccc9fb27dd561911ce69d81f34adec4c811`. Source reference: [ArtalkJS/Artalk](https://github.com/ArtalkJS/Artalk).

```text
The MIT License (MIT)

Copyright (c) 2018-present, qwqcode and other contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

## MinIO and MinIO Client

`deploy/minio/Dockerfile` builds MinIO and MinIO Client (`mc`) from immutable upstream commits. MinIO is licensed under the GNU Affero General Public License v3.0; `mc` is licensed under the GNU General Public License v3.0. The corresponding source revisions are recorded directly in that Dockerfile:

- MinIO: `9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a`;
- MinIO Client: `77f82e18b5401a65958f1619df6ebb994634bd88`.

The built image includes both upstream license files under `/licenses`. Source repositories:

- https://github.com/minio/minio
- https://github.com/minio/mc

## gosu

`deploy/mysql/Dockerfile` rebuilds gosu 1.19 from immutable upstream commit
`6456aaa0f3c854d199d0f037f068eb97515b7513` with the pinned Go security
toolchain. Its MIT license is copied into the image under `/licenses/gosu`.
Source repository: https://github.com/tianon/gosu
