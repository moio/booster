# regsync

## Problem

Copy a set of images from a Registry ("master") into another Registry ("replica"), reducing bandwidth usage compared to current solutions.

## Solution guideline

Use delta encoding for transfer, to exploit the fact that updated images potentially have layer content which is similar to their previous versions.

Additionally, un-apply `gzip` compression at the sender side and re-apply it at the receiver, so that delta encoding can work more efficiently (on uncompressed data).

## Premise: lossless gzip decompression/recompression

Using the exact same algorithm/implementation, `gzip` layers can be decompressed and recompressed to obtain exactly the same original file.

Assumption: most images out there have been created using go's `gzip` implementation, which has this property.

## Premise: the original rsync algorithm

- basic assumption: a single destination file exists on a destination host, similar but not identical to a source file on a source host
- the destination host applies the `CreateSignature` function to the destination file. Result is a list of hashes of the file's blocks ("signature")
- signature is sent to the source host
- source host applies the `CreateDelta` function, which takes as input the signature and the source file. Result is a list of operations ("delta")
    - crucially, one such operation can be: reuse a blob taken from the existing destination file at a certain position
- delta is sent to destination host
- destination host applies the `ApplyDelta` function, which takes as input the delta and the destination file. Result is an updated destination file which is identical to the source

## Premise: the extended rsync algorithm

- basic assumption: a **directory structure** exists on a destination host, similar but not identical to a directory structure on a source host
- the destination host applies the `CreateSignature` function to all files in its directory. Result is a **map from filenames to signatures**
- signatures are sent to the source host
- source host applies the `CreateDeltaEx` function, which takes as input the signatures and the source directory. Result is a **map from filenames to operations** ("deltas")
    - crucially, one such operation can be: reuse a blob taken **from a particular destination file**
- deltas are sent to destination host
- destination host applies the `ApplyDeltaEx` function, which takes as input the deltas and the destination direcrory. Result is an updated destination direcrory which is identical to the source

## Solution description

1) Uncompress all `gzip`-ed layers both on the sending and on the receiving Registries, saving them with a `.decompressed` filename suffix. Retain originals

2) Apply the extended rsync algorithm to transfer decompressed image files (and any other files). Use a compressed transport (eg. `Content-Encoding: deflate` HTTP)

3) recompress any changed `.decompressed` files at the destination

## Details

 - only create `.decompressed` files when it is sure they can be losslessly recompressed (ie. their recompressed version has the same hash). This can be checked at decompression time
 - one new HTTP endpoint needs to be created, to take signatures as input and provide deltas as output

https://en.wikipedia.org/wiki/Rsync#Algorithm