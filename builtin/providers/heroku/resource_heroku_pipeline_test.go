package heroku

import (
	"context"
	"fmt"
	"testing"

	heroku "github.com/cyberdelia/heroku-go/v3"
	"github.com/hashicorp/terraform/helper/acctest"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

func TestAccHerokuPipeline_Basic(t *testing.T) {
	var pipeline heroku.PipelineInfoResult
	pipelineName := fmt.Sprintf("tftest-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckHerokuPipelineDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCheckHerokuPipelineConfig_basic(pipelineName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckHerokuPipelineExists("heroku_pipeline.foobar", &pipeline),
					testAccCheckHerokuPipelineAttributes(&pipeline, pipelineName),
				),
			},
		},
	})
}

func testAccCheckHerokuPipelineConfig_basic(pipelineName string) string {
	return fmt.Sprintf(`
resource "heroku_pipeline" "foobar" {
  name = "%s"
}
`, pipelineName)
}

func testAccCheckHerokuPipelineExists(n string, pipeline *heroku.PipelineInfoResult) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]

		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No pipeline name set")
		}

		client := testAccProvider.Meta().(*heroku.Service)

		foundPipeline, err := client.PipelineInfo(context.TODO(), rs.Primary.ID)
		if err != nil {
			return err
		}

		if foundPipeline.ID != rs.Primary.ID {
			return fmt.Errorf("Pipeline not found")
		}

		*pipeline = *foundPipeline

		return nil
	}
}

func testAccCheckHerokuPipelineAttributes(pipeline *heroku.PipelineInfoResult, pipelineName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if pipeline.Name != pipelineName {
			return fmt.Errorf("Bad name: %s", pipeline.Name)
		}

		return nil
	}
}

func testAccCheckHerokuPipelineDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*heroku.Service)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "heroku_pipeline" {
			continue
		}

		_, err := client.PipelineInfo(context.TODO(), rs.Primary.ID)

		if err == nil {
			return fmt.Errorf("Pipeline still exists")
		}
	}

	return nil
}
