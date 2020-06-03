---
layout: "docs"
page_title: "Terraform Settings - Configuration Language"
sidebar_current: "docs-config-terraform"
description: |-
  The "terraform" configuration section is used to configure some behaviors
  of Terraform itself.
---

# Terraform Settings

-> **Note:** This page is about Terraform 0.12 and later. For Terraform 0.11 and
earlier, see
[0.11 Configuration Language: Terraform Settings](../configuration-0-11/terraform.html).

The special `terraform` configuration block type is used to configure some
behaviors of Terraform itself, such as requiring a minimum Terraform version to
apply your configuration.

## Terraform Block Syntax

Terraform-specific settings are gathered together into `terraform` blocks:

```hcl
terraform {
  # ...
}
```

Each `terraform` block can contain a number of settings related to Terraform's
behavior. Within a `terraform` block, only constant values can be used;
arguments may not refer to named objects such as resources, input variables,
etc, and may not use any of the Terraform language built-in functions.

The various options supported within a `terraform` block are described in the
following sections.

## Configuring a Terraform Backend

The nested `backend` block configures which backend Terraform should use.

The syntax and behavior of the `backend` block is described in [Backend
Configuration](./backend.html).

## Specifying a Required Terraform Version

The `required_version` setting accepts a [version constraint
string,](./version-constraints.html) which specifies which versions of Terraform
can be used with your configuration.

If the running version of Terraform doesn't match the constraints specified,
Terraform will produce an error and exit without taking any further actions.

When you use [child modules](./modules.html), each module can specify its own
version requirements. The requirements of all modules in the tree must be
satisfied.

Use Terraform version constraints in a collaborative environment to
ensure that everyone is using a specific Terraform version, or using at least
a minimum Terraform version that has behavior expected by the configuration.

The `required_version` setting applies only to the version of Terraform CLI.
Various behaviors of Terraform are actually implemented by Terraform providers,
which are released on a cycle independent of Terraform CLI and of each other.
Use [provider version constraints](./providers.html#provider-versions)
to make similar constraints on which provider versions may be used.

## Specifying Required Provider Versions and Source

[inpage-source]: #specifying-required-provider-versions-and-source

-> **Note:** The provider `source` attribute was introduced in Terraform v0.13.

The `required_providers` setting is a map specifying a version constraint and source for
each provider required by your configuration.

```hcl
terraform {
  required_providers {
    aws = {
      version = ">= 2.7.0"
      source = "hashicorp/aws"
    }
  }
}
```

You may omit the `source` attribute for providers in the `hashicorp` namespace.
In those cases, an optional, simplified syntax may also be used:

```hcl
terraform {
  required_providers {
    aws = ">= 2.7.0"
  }
}
```

### Third-Party Providers

If you have a third-party provider that is not in the public registry,
you will need to make up an arbitrary source for that provider and copy (or
link) the binary to a directory corresponding to that source.

Once you've chosen a source, the binary needs to be installed into the following directory heirarchy:

```
$PLUGINDIR/$SOURCEHOST/$NAMESPACE/$TYPE/$VERSION/$OS_$ARCH/
```

The $OS_$ARCH must be the same operating system and architecture you are
currently using for Terraform.

For example, consider a provider called `terraform-provider-mycloud`. You can
use any source, though a best practice is to choose something logical to you:

```hcl
terraform {
  required_providers {
    mycloud = {
      source = "example.com/mycompany/mycloud"
      version = "1.0"
    }
  }
}
```

Terraform will look for the binary in the following directory (replace `$OS_$ARCH` with the appropriate operating system and architecture which you are using to run Terraform):

```
$PLUGINDIR/example.com/mycompany/mycloud/1.0/$OS_$ARCH/terraform-provider-mycloud
```

### Version Constraint Strings

Version constraint strings within the `required_providers` block use the
same version constraint syntax as for
[the `required_version` argument](#specifying-a-required-terraform-version)
described above.

When a configuration contains multiple version constraints for a single
provider -- for example, if you're using multiple modules and each one has
its own constraint -- _all_ of the constraints must hold to select a single
provider version for the whole configuration.

Re-usable modules should constrain only the minimum allowed version, such
as `>= 1.0.0`. This specifies the earliest version that the module is
compatible with while leaving the user of the module flexibility to upgrade
to newer versions of the provider without altering the module.

Root modules should use a `~>` constraint to set both a lower and upper bound
on versions for each provider they depend on, as described in
[Provider Versions](providers.html#provider-versions).

### Source Constraint Strings

A source constraint string within the `required_providers` is a string made up
of one to three parts, separated by a forward-slash (`/`). The parts are:

* `hostname`: The `hostname` is the registry host which indexes the provider.
  `hostname` may be omitted if the provider is in HashiCorp's public registry
  (`registry.terraform.io`).

* `namespace`: The registry namespace that the provider is in. This may be
  omitted if the provider is in HashiCorp's namesapce (`hashicorp`). `namespace`
  is required when `hostname` is set.

* `type`: The provider type.


The following are all valid source strings for the `random` provider in the
HashiCorp namespace:
```
"random"
"hashicorp/random"
"registry.terraform.io/hashicorp/random"
```

The following is _not_ a valid source string, since namespace is required when
hostname is provided:
```
"registry.terraform.io/random"
```

## Experimental Language Features

From time to time the Terraform team will introduce new language features
initially via an opt-in experiment, so that the community can try the new
feature and give feedback on it prior to it becoming a backward-compatibility
constraint.

In releases where experimental features are available, you can enable them on
a per-module basis by setting the `experiments` argument inside a `terraform`
block:

```hcl
terraform {
  experiments = [example]
}
```

The above would opt in to an experiment named `example`, assuming such an
experiment were available in the current Terraform version.

Experiments are subject to arbitrary changes in later releases and, depending on
the outcome of the experiment, may change drastically before final release or
may not be released in stable form at all. Such breaking changes may appear
even in minor and patch releases. We do not recommend using experimental
features in Terraform modules intended for production use.

In order to make that explicit and to avoid module callers inadvertently
depending on an experimental feature, any module with experiments enabled will
generate a warning on every `terraform plan` or `terraform apply`. If you
want to try experimental features in a shared module, we recommend enabling the
experiment only in alpha or beta releases of the module.

The introduction and completion of experiments is reported in
[Terraform's changelog](https://github.com/hashicorp/terraform/blob/master/CHANGELOG.md),
so you can watch the release notes there to discover which experiment keywords,
if any, are available in a particular Terraform release.

## Passing Metadata to Providers

The `terraform` block can have a nested `provider_meta` block for each
provider a module is using, if the provider defines a schema for it. This
allows the provider to receive module-specific information. No interpolations
are performed on this block. For more information, see the
[`provider_meta` page](/docs/internals/provider-meta.html).
