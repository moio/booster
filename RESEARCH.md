# Research results (Linux)

Linux:
- Rancher 2.5.8's images (list below) take ~8 GB/~23 GB uncompressed
- patch creation takes ~50 minutes (of which ~5 minutes decompression)
- patch application takes ~9 minutes (of which ~3 minutes recompression)
- patch from "nothing" to "2.5.8" takes ~6 GB (~78% of `docker pull` size)

- rancher 2.5.9's images take similar space (~8 GB/~23 GB uncompressed)
- updating from 2.5.8's images adds another ~2 GB/~7 GB uncompressed
- patch creation takes ~27 minutes (of which ~1 minute decompression)
- patch application takes ~2 minutes (of which ~1 minute recompression)
- patch "2.5.8 -> 2.5.9" takes ~882 MB (~46% of `docker pull` size)

- rancher 2.6.0's images take a bit more space (~9 GB/~27 GB uncompressed)
- updating from 2.5.9's images adds another ~6 GB/~17 GB uncompressed
- patch creation takes ~1 hour (of which ~3 minutes decompression)
- patch application takes ~7 minutes (of which ~3 minute recompression)
- patch "2.5.9 -> 2.6.0" takes ~4 GB (~62% of `docker pull` size)

Windows (20H2 tested):
- rancher 2.5.9's images take ~3 GB (~8 GB uncompressed)
- patch creation takes ~13 minutes (of which ~3 minutes decompression, CPU bound)
- patch application takes ~2 minutes (of which ~1 minute recompression, CPU bound)
- patch from "nothing" to "2.5.9" takes ~2 GB (~80% of `docker pull` size)

- rancher 2.6.0's images takes similar space (~3 GB/~8 GB uncompressed)
- updating from 2.5.9's images adds another ~2 GB compressed
- patch creation takes 13~ minutes (of which ~1 minute decompression)
- patch application takes ~2 minutes (of which 1~ minute recompression)
- patch "2.5.9 -> 2.6.0" takes ~1 GB (~63% of `docker pull` size)

Windows (more than one version):
- adding rancher 2.5.9-windows-20H2 on top of 2.5.9-windows-2004 takes ~1.7 GB
    - respective patch takes 569 MB (~33% of `docker pull` size)
- adding rancher 2.6.0-windows-20H2 on top of 2.6.0-windows-2004 takes ~1.2 GB
    - respective patch takes 172 MB (~13% of `docker pull` size)

With bdiff optimization: 550MB patch saw a ~20% size reduction with ~1.5 h extra processing time

## Test snippets

Linux:
```shell
VERSION=2.5.8
curl --location -o rancher-images-v$VERSION.txt https://github.com/rancher/rancher/releases/download/v$VERSION/rancher-images.txt

for image in `cat rancher-images-v$VERSION.txt`
do
    docker pull $image
done

for image in `cat rancher-images-v$VERSION.txt`
do
    docker image tag $image localhost:5001/$image
    docker image push localhost:5001/$image
done

curl http://localhost:5004/sync

echo Compressed size:
du -acBM `find primary/docker/ -type f  | grep -v "UNGZIPPED_BY_BOOSTER"` | tail -1
echo Uncompressed size:
du -acBM `find primary/docker/ -type f  | grep "UNGZIPPED_BY_BOOSTER"` | tail -1
echo Patch size:
sudo du -acBM `sudo find primary/booster/ -type f`
```

```shell
VERSION=2.5.8
WINDOWS_VERSION=20H2
curl --location -o rancher-windows-images-v$VERSION.txt https://github.com/rancher/rancher/releases/download/v$VERSION/rancher-windows-images.txt

for image in `cat rancher-windows-images-v$VERSION.txt`
do
    skopeo copy --dest-tls-verify=false docker://$image-windows-$WINDOWS_VERSION docker://localhost:5001/$image-windows-$WINDOWS_VERSION
done
```
