#!/bin/bash

set -o errexit -o nounset

docker run --rm -it \
  -e ARM_CLIENT_ID \
  -e ARM_CLIENT_SECRET \
  -e ARM_SUBSCRIPTION_ID \
  -e ARM_TENANT_ID \
  -v $(pwd):/data \
  --workdir=/data \
  --entrypoint "/bin/sh" \
  hashicorp/terraform:light \
  -c "/bin/terraform get; \
      /bin/terraform validate; \
      /bin/terraform plan -out=out.tfplan -var unique=$KEY -var resource_group=$EXISTING_RESOURCE_GROUP; \
      /bin/terraform apply out.tfplan \
      /bin/terraform show;"


# cleanup deployed azure resources via terraform
docker run --rm -it \
  -e ARM_CLIENT_ID \
  -e ARM_CLIENT_SECRET \
  -e ARM_SUBSCRIPTION_ID \
  -e ARM_TENANT_ID \
  -v $(pwd):/data \
  --workdir=/data \
  --entrypoint "/bin/sh" \
  hashicorp/terraform:light \
  -c "/bin/terraform destroy -force -var unique=$KEY -var resource_group=$KEY;"


# cleanup deployed azure resources via azure-cli
# docker run --rm -it \
#   azuresdk/azure-cli-python \
#   sh -c "az login --service-principal -u $ARM_CLIENT_ID -p $ARM_CLIENT_SECRET --tenant $ARM_TENANT_ID; \
#          az resource delete --resource-group permanent;"