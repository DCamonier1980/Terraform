variable "existing_vnet_resource_group" {
  description = "Name of the existing VNET resource group in which the existing vnet resides"
}

variable "location" {
  description = "The location/region where the virtual network resides."
  default     = "southcentralus"
}

variable "dns_name" {
  description = "Unique DNS Name for the Public IP used to access the Virtual Machine. Will be used to make up the FQDN. Must be globally unique."
}

variable "os_type" {
  description = "Type of OS on the existing vhd. Allowed values: 'Windows' or 'Linux'."
  default     = "Linux"
}

variable "os_disk_vhd_uri" {
  description = "Uri of the existing VHD in ARM standard or premium storage"
  default     = "https://permanentstor.blob.core.windows.net/permanent-vhds/permanent-osdisk1.vhd"
}

variable "existing_virtual_network_name" {
  description = "The name for the existing virtual network"
  default     = "vnet"
}

variable "subnet_name" {
  description = "Name of the subnet in the virtual network you want to use"
}

variable "address_space" {
  description = "The address space that is used by the virtual network. You can supply more than one address space. Changing this forces a new resource to be created."
  default     = "10.0.0.0/16"
}

variable "subnet_prefix" {
  description = "The address prefix to use for the subnet."
  default     = "10.0.10.0/24"
}

variable "storage_account_type" {
  description = "Defines the type of storage account to be created. Valid options are Standard_LRS, Standard_ZRS, Standard_GRS, Standard_RAGRS, Premium_LRS. Changing this is sometimes valid - see the Azure documentation for more information on which types of accounts can be converted into other types."
  default     = "Standard_LRS"
}

variable "vm_size" {
  description = "Specifies the size of the virtual machine."
  default     = "Standard_A0"
}

variable "image_publisher" {
  description = "name of the publisher of the image (az vm image list)"
  default     = "Canonical"
}

variable "image_offer" {
  description = "the name of the offer (az vm image list)"
  default     = "UbuntuServer"
}

variable "image_sku" {
  description = "image sku to apply (az vm image list)"
  default     = "16.04-LTS"
}

variable "image_version" {
  description = "version of the image to apply (az vm image list)"
  default     = "latest"
}

variable "admin_username" {
  description = "administrator user name"
  default     = "vmadmin"
}

variable "admin_password" {
  description = "administrator password (recommended to disable password auth)"
}
    # "vnetID": "[resourceId(parameters('existingVirtualNetworkResourceGroup'), 'Microsoft.Network/virtualNetworks', parameters('existingVirtualNetworkName'))]",
    # "subnetRef": "[concat(variables('vnetID'),'/subnets/', parameters('subnetName'))]"