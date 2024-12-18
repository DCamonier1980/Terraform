mock_provider "test" {
  alias = "primary"
  override_computed = foo // This should be a boolean value, therefore this test should fail

  mock_resource "test_resource" {
    defaults = {
      id = "aaaa"
    }
  }

  override_resource {
    target = test_resource.primary
    values = {
      id = "bbbb"
    }
  }
}

variables {
  instances = 1
  child_instances = 1
}

run "test" {

  assert {
    condition = test_resource.primary[0].id == "bbbb"
    error_message = "mock not applied"
  }
}
