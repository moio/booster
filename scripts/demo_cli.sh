# Start a registry on port 5001
docker run -d \
  -p 5001:5000 \
  -v `pwd`/registry:/var/lib/registry \
  registry:2

# Load an old version of Ubuntu into the registry
docker pull ubuntu:bionic-20210615.1
docker image tag ubuntu:bionic-20210615.1 localhost:5001/ubuntu:bionic-20210615.1
docker image push localhost:5001/ubuntu:bionic-20210615.1

# Create a patch to the new version
echo ubuntu:bionic-20210615.1 > old.txt
echo ubuntu:bionic-20210702 > new.txt

booster diff old.txt new.txt

# Apply the patch onto the registry
booster apply old.txt new.txt old-to-new.patch localhost:5001
