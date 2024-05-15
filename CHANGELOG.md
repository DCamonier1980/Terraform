## 1.9.0 (Unreleased)

ENHANCEMENTS:

* `terraform console`: Now has basic support for multi-line input in interactive mode. ([#34822](https://github.com/hashicorp/terraform/pull/34822))

    If an entered line contains opening parentheses/etc that are not closed, Terraform will await another line of input to complete the expression. This initial implementation is primarily intended to support pasting in multi-line expressions from elsewhere, rather than for manual multi-line editing, so the interactive editing support is currently limited.
* `terraform plan`: Improved presentation of OPA and Sentinel policy evaluations in HCP Terraform remote runs, for logical separation.
* `terraform init` now accepts a `-json` option. If specified, enables the machine readable JSON output. ([#34886](https://github.com/hashicorp/terraform/pull/34886))
* `terraform test`: Test runs can now pass sensitive values to input variables while preserving their dynamic sensitivity. Previously sensitivity would be preserved only for variables statically declared as being sensitive, using `sensitive = true`. ([#35021](https://github.com/hashicorp/terraform/pull/35021))
* Performance improvement during graph building for configurations with an extremely large number of `resource` blocks. ([#35088](https://github.com/hashicorp/terraform/pull/35088))

BUG FIXES:

* `remote-exec`: Each remote connection will now be closed immediately after use. ([#34137](https://github.com/hashicorp/terraform/issues/34137))
* `backend/s3`: Fixed the digest value displayed for DynamoDB/S3 state checksum mismatches. ([#34387](https://github.com/hashicorp/terraform/issues/34387))

EXPERIMENTS:

Experiments are only enabled in alpha releases of Terraform CLI. The following features are not yet available in stable releases.

* `variable_validation_crossref`: This [language experiment](https://developer.hashicorp.com/terraform/language/settings#experimental-language-features) allows `validation` blocks inside input variable declarations to refer to other objects inside the module where the variable is declared, including to the values of other input variables in the same module.
* `template_string_func`: This [language experiment](https://developer.hashicorp.com/terraform/language/settings#experimental-language-features) introduces a new built-in function named `templatestring` which is similar to `templatefile` but designed to render templates obtained dynamically, such as from a data resource result.
* `terraform test` accepts a new option `-junit-xml=FILENAME`. If specified, and if the test configuration is valid enough to begin executing, then Terraform writes a JUnit XML test result report to the given filename, describing similar information as included in the normal test output. ([#34291](https://github.com/hashicorp/terraform/issues/34291))
* The new command `terraform rpcapi` exposes some Terraform Core functionality through an RPC interface compatible with [`go-plugin`](https://github.com/hashicorp/go-plugin). The exact RPC API exposed here is currently subject to change at any time, because it's here primarily as a vehicle to support the [Terraform Stacks](https://www.hashicorp.com/blog/terraform-stacks-explained) private preview and so will be broken if necessary to respond to feedback from private preview participants, or possibly for other reasons. Do not use this mechanism yet outside of Terraform Stacks private preview.
* The experimental "deferred actions" feature, enabled by passing the `-allow-deferral` option to `terraform plan`, permits `count` and `for_each` arguments in `module`, `resource`, and `data` blocks to have unknown values and allows providers to react more flexibly to unknown values. This experiment is under active development, and so it's not yet useful to participate in this experiment.

## Previous Releases

For information on prior major and minor releases, see their changelogs:

* [v1.8](https://github.com/hashicorp/terraform/blob/v1.8/CHANGELOG.md)
* [v1.7](https://github.com/hashicorp/terraform/blob/v1.7/CHANGELOG.md)
* [v1.6](https://github.com/hashicorp/terraform/blob/v1.6/CHANGELOG.md)
* [v1.5](https://github.com/hashicorp/terraform/blob/v1.5/CHANGELOG.md)
* [v1.4](https://github.com/hashicorp/terraform/blob/v1.4/CHANGELOG.md)
* [v1.3](https://github.com/hashicorp/terraform/blob/v1.3/CHANGELOG.md)
* [v1.2](https://github.com/hashicorp/terraform/blob/v1.2/CHANGELOG.md)
* [v1.1](https://github.com/hashicorp/terraform/blob/v1.1/CHANGELOG.md)
* [v1.0](https://github.com/hashicorp/terraform/blob/v1.0/CHANGELOG.md)
* [v0.15](https://github.com/hashicorp/terraform/blob/v0.15/CHANGELOG.md)
* [v0.14](https://github.com/hashicorp/terraform/blob/v0.14/CHANGELOG.md)
* [v0.13](https://github.com/hashicorp/terraform/blob/v0.13/CHANGELOG.md)
* [v0.12](https://github.com/hashicorp/terraform/blob/v0.12/CHANGELOG.md)
* [v0.11 and earlier](https://github.com/hashicorp/terraform/blob/v0.11/CHANGELOG.md)
