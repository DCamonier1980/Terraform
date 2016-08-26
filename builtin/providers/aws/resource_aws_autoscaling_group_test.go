package aws

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/hashicorp/terraform/helper/acctest"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

func TestAccAWSAutoScalingGroup_basic(t *testing.T) {
	var group autoscaling.Group
	var lc autoscaling.LaunchConfiguration

	randName := fmt.Sprintf("terraform-test-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:        func() { testAccPreCheck(t) },
		IDRefreshName:   "aws_autoscaling_group.bar",
		IDRefreshIgnore: []string{"force_delete", "metrics_granularity", "wait_for_capacity_timeout"},
		Providers:       testAccProviders,
		CheckDestroy:    testAccCheckAWSAutoScalingGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig(randName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					testAccCheckAWSAutoScalingGroupHealthyCapacity(&group, 2),
					testAccCheckAWSAutoScalingGroupAttributes(&group, randName),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "availability_zones.2487133097", "us-west-2a"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "name", randName),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "max_size", "5"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "min_size", "2"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "health_check_grace_period", "300"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "health_check_type", "ELB"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "desired_capacity", "4"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "force_delete", "true"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "termination_policies.0", "OldestInstance"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "termination_policies.1", "ClosestToNextInstanceHour"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "protect_from_scale_in", "false"),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfigUpdate(randName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					testAccCheckAWSLaunchConfigurationExists("aws_launch_configuration.new", &lc),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "desired_capacity", "5"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "termination_policies.0", "ClosestToNextInstanceHour"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "protect_from_scale_in", "true"),
					testLaunchConfigurationName("aws_autoscaling_group.bar", &lc),
					testAccCheckAutoscalingTags(&group.Tags, "Bar", map[string]interface{}{
						"value":               "bar-foo",
						"propagate_at_launch": true,
					}),
				),
			},
		},
	})
}

func TestAccAWSAutoScalingGroup_autoGeneratedName(t *testing.T) {
	asgNameRegexp := regexp.MustCompile("^tf-asg-")

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSAutoScalingGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig_autoGeneratedName,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr(
						"aws_autoscaling_group.bar", "name", asgNameRegexp),
					resource.TestCheckResourceAttrSet(
						"aws_autoscaling_group.bar", "arn"),
				),
			},
		},
	})
}

func TestAccAWSAutoScalingGroup_terminationPolicies(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSAutoScalingGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig_terminationPoliciesEmpty,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "termination_policies.#", "0"),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig_terminationPoliciesUpdate,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "termination_policies.#", "1"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "termination_policies.0", "OldestInstance"),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig_terminationPoliciesExplicitDefault,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "termination_policies.#", "1"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "termination_policies.0", "Default"),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig_terminationPoliciesEmpty,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "termination_policies.#", "0"),
				),
			},
		},
	})
}

func TestAccAWSAutoScalingGroup_tags(t *testing.T) {
	var group autoscaling.Group

	randName := fmt.Sprintf("tfautotags-%s", acctest.RandString(5))

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSAutoScalingGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig(randName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					testAccCheckAutoscalingTags(&group.Tags, "Foo", map[string]interface{}{
						"value":               "foo-bar",
						"propagate_at_launch": true,
					}),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfigUpdate(randName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					testAccCheckAutoscalingTagNotExists(&group.Tags, "Foo"),
					testAccCheckAutoscalingTags(&group.Tags, "Bar", map[string]interface{}{
						"value":               "bar-foo",
						"propagate_at_launch": true,
					}),
				),
			},
		},
	})
}

func TestAccAWSAutoScalingGroup_VpcUpdates(t *testing.T) {
	var group autoscaling.Group

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSAutoScalingGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfigWithAZ,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "availability_zones.#", "1"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "availability_zones.2487133097", "us-west-2a"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "vpc_zone_identifier.#", "1"),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfigWithVPCIdent,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					testAccCheckAWSAutoScalingGroupAttributesVPCZoneIdentifer(&group),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "availability_zones.#", "1"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "availability_zones.2487133097", "us-west-2a"),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "vpc_zone_identifier.#", "1"),
				),
			},
		},
	})
}

func TestAccAWSAutoScalingGroup_WithLoadBalancer(t *testing.T) {
	var group autoscaling.Group

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSAutoScalingGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfigWithLoadBalancer,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					testAccCheckAWSAutoScalingGroupAttributesLoadBalancer(&group),
				),
			},
		},
	})
}

func TestAccAWSAutoScalingGroup_withPlacementGroup(t *testing.T) {
	var group autoscaling.Group

	randName := fmt.Sprintf("tf_placement_test-%s", acctest.RandString(5))
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSAutoScalingGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig_withPlacementGroup(randName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "placement_group", randName),
				),
			},
		},
	})
}

func TestAccAWSAutoScalingGroup_enablingMetrics(t *testing.T) {
	var group autoscaling.Group
	randName := fmt.Sprintf("terraform-test-%s", acctest.RandString(10))

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSAutoScalingGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig(randName),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "enabled_metrics.#", ""),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoscalingMetricsCollectionConfig_updatingMetricsCollected,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "enabled_metrics.#", "5"),
				),
			},
		},
	})
}

func TestAccAWSAutoScalingGroup_withMetrics(t *testing.T) {
	var group autoscaling.Group

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSAutoScalingGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSAutoscalingMetricsCollectionConfig_allMetricsCollected,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "enabled_metrics.#", "7"),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoscalingMetricsCollectionConfig_updatingMetricsCollected,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "enabled_metrics.#", "5"),
				),
			},
		},
	})
}

func TestAccAWSAutoScalingGroup_ALB_TargetGroups(t *testing.T) {
	var group autoscaling.Group
	var tg elbv2.TargetGroup
	var tg2 elbv2.TargetGroup

	testCheck := func(targets []*elbv2.TargetGroup) resource.TestCheckFunc {
		return func(*terraform.State) error {
			var ts []string
			var gs []string
			for _, t := range targets {
				ts = append(ts, *t.TargetGroupArn)
			}

			for _, s := range group.TargetGroupARNs {
				gs = append(gs, *s)
			}

			sort.Strings(ts)
			sort.Strings(gs)

			if !reflect.DeepEqual(ts, gs) {
				return fmt.Errorf("Error: target group match not found!\nASG Target groups: %#v\nTarget Group: %#v", ts, gs)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSAutoScalingGroupDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig_ALB_TargetGroup_pre,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					testAccCheckAWSALBTargetGroupExists("aws_alb_target_group.test", &tg),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "target_group_arns.#", "0"),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig_ALB_TargetGroup_post_duo,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					testAccCheckAWSALBTargetGroupExists("aws_alb_target_group.test", &tg),
					testAccCheckAWSALBTargetGroupExists("aws_alb_target_group.test_more", &tg2),
					testCheck([]*elbv2.TargetGroup{&tg, &tg2}),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "target_group_arns.#", "2"),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig_ALB_TargetGroup_post,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					testAccCheckAWSALBTargetGroupExists("aws_alb_target_group.test", &tg),
					testCheck([]*elbv2.TargetGroup{&tg}),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "target_group_arns.#", "1"),
				),
			},

			resource.TestStep{
				Config: testAccAWSAutoScalingGroupConfig_ALB_TargetGroup_pre,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAWSAutoScalingGroupExists("aws_autoscaling_group.bar", &group),
					resource.TestCheckResourceAttr(
						"aws_autoscaling_group.bar", "target_group_arns.#", "0"),
				),
			},
		},
	})
}

func testAccCheckAWSAutoScalingGroupDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*AWSClient).autoscalingconn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_autoscaling_group" {
			continue
		}

		// Try to find the Group
		describeGroups, err := conn.DescribeAutoScalingGroups(
			&autoscaling.DescribeAutoScalingGroupsInput{
				AutoScalingGroupNames: []*string{aws.String(rs.Primary.ID)},
			})

		if err == nil {
			if len(describeGroups.AutoScalingGroups) != 0 &&
				*describeGroups.AutoScalingGroups[0].AutoScalingGroupName == rs.Primary.ID {
				return fmt.Errorf("AutoScaling Group still exists")
			}
		}

		// Verify the error
		ec2err, ok := err.(awserr.Error)
		if !ok {
			return err
		}
		if ec2err.Code() != "InvalidGroup.NotFound" {
			return err
		}
	}

	return nil
}

func testAccCheckAWSAutoScalingGroupAttributes(group *autoscaling.Group, name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if *group.AvailabilityZones[0] != "us-west-2a" {
			return fmt.Errorf("Bad availability_zones: %#v", group.AvailabilityZones[0])
		}

		if *group.AutoScalingGroupName != name {
			return fmt.Errorf("Bad Autoscaling Group name, expected (%s), got (%s)", name, *group.AutoScalingGroupName)
		}

		if *group.MaxSize != 5 {
			return fmt.Errorf("Bad max_size: %d", *group.MaxSize)
		}

		if *group.MinSize != 2 {
			return fmt.Errorf("Bad max_size: %d", *group.MinSize)
		}

		if *group.HealthCheckType != "ELB" {
			return fmt.Errorf("Bad health_check_type,\nexpected: %s\ngot: %s", "ELB", *group.HealthCheckType)
		}

		if *group.HealthCheckGracePeriod != 300 {
			return fmt.Errorf("Bad health_check_grace_period: %d", *group.HealthCheckGracePeriod)
		}

		if *group.DesiredCapacity != 4 {
			return fmt.Errorf("Bad desired_capacity: %d", *group.DesiredCapacity)
		}

		if *group.LaunchConfigurationName == "" {
			return fmt.Errorf("Bad launch configuration name: %s", *group.LaunchConfigurationName)
		}

		t := &autoscaling.TagDescription{
			Key:               aws.String("Foo"),
			Value:             aws.String("foo-bar"),
			PropagateAtLaunch: aws.Bool(true),
			ResourceType:      aws.String("auto-scaling-group"),
			ResourceId:        group.AutoScalingGroupName,
		}

		if !reflect.DeepEqual(group.Tags[0], t) {
			return fmt.Errorf(
				"Got:\n\n%#v\n\nExpected:\n\n%#v\n",
				group.Tags[0],
				t)
		}

		return nil
	}
}

func testAccCheckAWSAutoScalingGroupAttributesLoadBalancer(group *autoscaling.Group) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if len(group.LoadBalancerNames) != 1 {
			return fmt.Errorf("Bad load_balancers: %v", group.LoadBalancerNames)
		}

		return nil
	}
}

func testAccCheckAWSAutoScalingGroupExists(n string, group *autoscaling.Group) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No AutoScaling Group ID is set")
		}

		conn := testAccProvider.Meta().(*AWSClient).autoscalingconn

		describeGroups, err := conn.DescribeAutoScalingGroups(
			&autoscaling.DescribeAutoScalingGroupsInput{
				AutoScalingGroupNames: []*string{aws.String(rs.Primary.ID)},
			})

		if err != nil {
			return err
		}

		if len(describeGroups.AutoScalingGroups) != 1 ||
			*describeGroups.AutoScalingGroups[0].AutoScalingGroupName != rs.Primary.ID {
			return fmt.Errorf("AutoScaling Group not found")
		}

		*group = *describeGroups.AutoScalingGroups[0]

		return nil
	}
}

func testLaunchConfigurationName(n string, lc *autoscaling.LaunchConfiguration) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if *lc.LaunchConfigurationName != rs.Primary.Attributes["launch_configuration"] {
			return fmt.Errorf("Launch configuration names do not match")
		}

		return nil
	}
}

func testAccCheckAWSAutoScalingGroupHealthyCapacity(
	g *autoscaling.Group, exp int) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		healthy := 0
		for _, i := range g.Instances {
			if i.HealthStatus == nil {
				continue
			}
			if strings.EqualFold(*i.HealthStatus, "Healthy") {
				healthy++
			}
		}
		if healthy < exp {
			return fmt.Errorf("Expected at least %d healthy, got %d.", exp, healthy)
		}
		return nil
	}
}

func testAccCheckAWSAutoScalingGroupAttributesVPCZoneIdentifer(group *autoscaling.Group) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// Grab Subnet Ids
		var subnets []string
		for _, rs := range s.RootModule().Resources {
			if rs.Type != "aws_subnet" {
				continue
			}
			subnets = append(subnets, rs.Primary.Attributes["id"])
		}

		if group.VPCZoneIdentifier == nil {
			return fmt.Errorf("Bad VPC Zone Identifier\nexpected: %s\ngot nil", subnets)
		}

		zones := strings.Split(*group.VPCZoneIdentifier, ",")

		remaining := len(zones)
		for _, z := range zones {
			for _, s := range subnets {
				if z == s {
					remaining--
				}
			}
		}

		if remaining != 0 {
			return fmt.Errorf("Bad VPC Zone Identifier match\nexpected: %s\ngot:%s", zones, subnets)
		}

		return nil
	}
}

const testAccAWSAutoScalingGroupConfig_autoGeneratedName = `
resource "aws_launch_configuration" "foobar" {
  image_id = "ami-21f78e11"
  instance_type = "t1.micro"
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = ["us-west-2a"]
  desired_capacity = 0
  max_size = 0
  min_size = 0
  launch_configuration = "${aws_launch_configuration.foobar.name}"
}
`

const testAccAWSAutoScalingGroupConfig_terminationPoliciesEmpty = `
resource "aws_launch_configuration" "foobar" {
  image_id = "ami-21f78e11"
  instance_type = "t1.micro"
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = ["us-west-2a"]
  max_size = 0
  min_size = 0
  desired_capacity = 0

  launch_configuration = "${aws_launch_configuration.foobar.name}"
}
`

const testAccAWSAutoScalingGroupConfig_terminationPoliciesExplicitDefault = `
resource "aws_launch_configuration" "foobar" {
  image_id = "ami-21f78e11"
  instance_type = "t1.micro"
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = ["us-west-2a"]
  max_size = 0
  min_size = 0
  desired_capacity = 0
  termination_policies = ["Default"]

  launch_configuration = "${aws_launch_configuration.foobar.name}"
}
`

const testAccAWSAutoScalingGroupConfig_terminationPoliciesUpdate = `
resource "aws_launch_configuration" "foobar" {
  image_id = "ami-21f78e11"
  instance_type = "t1.micro"
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = ["us-west-2a"]
  max_size = 0
  min_size = 0
  desired_capacity = 0
  termination_policies = ["OldestInstance"]

  launch_configuration = "${aws_launch_configuration.foobar.name}"
}
`

func testAccAWSAutoScalingGroupConfig(name string) string {
	return fmt.Sprintf(`
resource "aws_launch_configuration" "foobar" {
  image_id = "ami-21f78e11"
  instance_type = "t1.micro"
}

resource "aws_placement_group" "test" {
  name = "asg_pg_%s"
  strategy = "cluster"
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = ["us-west-2a"]
  name = "%s"
  max_size = 5
  min_size = 2
  health_check_type = "ELB"
  desired_capacity = 4
  force_delete = true
  termination_policies = ["OldestInstance","ClosestToNextInstanceHour"]

  launch_configuration = "${aws_launch_configuration.foobar.name}"

  tag {
    key = "Foo"
    value = "foo-bar"
    propagate_at_launch = true
  }
}
`, name, name)
}

func testAccAWSAutoScalingGroupConfigUpdate(name string) string {
	return fmt.Sprintf(`
resource "aws_launch_configuration" "foobar" {
  image_id = "ami-21f78e11"
  instance_type = "t1.micro"
}

resource "aws_launch_configuration" "new" {
  image_id = "ami-21f78e11"
  instance_type = "t1.micro"
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = ["us-west-2a"]
  name = "%s"
  max_size = 5
  min_size = 2
  health_check_grace_period = 300
  health_check_type = "ELB"
  desired_capacity = 5
  force_delete = true
  termination_policies = ["ClosestToNextInstanceHour"]
  protect_from_scale_in = true

  launch_configuration = "${aws_launch_configuration.new.name}"

  tag {
    key = "Bar"
    value = "bar-foo"
    propagate_at_launch = true
  }
}
`, name)
}

const testAccAWSAutoScalingGroupConfigWithLoadBalancer = `
resource "aws_vpc" "foo" {
  cidr_block = "10.1.0.0/16"
	tags { Name = "tf-asg-test" }
}

resource "aws_internet_gateway" "gw" {
  vpc_id = "${aws_vpc.foo.id}"
}

resource "aws_subnet" "foo" {
	cidr_block = "10.1.1.0/24"
	vpc_id = "${aws_vpc.foo.id}"
}

resource "aws_security_group" "foo" {
  vpc_id="${aws_vpc.foo.id}"

  ingress {
    protocol = "-1"
    from_port = 0
    to_port = 0
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    protocol = "-1"
    from_port = 0
    to_port = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_elb" "bar" {
  subnets = ["${aws_subnet.foo.id}"]
	security_groups = ["${aws_security_group.foo.id}"]

  listener {
    instance_port = 80
    instance_protocol = "http"
    lb_port = 80
    lb_protocol = "http"
  }

  health_check {
    healthy_threshold = 2
    unhealthy_threshold = 2
    target = "HTTP:80/"
    interval = 5
    timeout = 2
  }

	depends_on = ["aws_internet_gateway.gw"]
}

resource "aws_launch_configuration" "foobar" {
  // need an AMI that listens on :80 at boot, this is:
  // bitnami-nginxstack-1.6.1-0-linux-ubuntu-14.04.1-x86_64-hvm-ebs-ami-99f5b1a9-3
  image_id = "ami-b5b3fc85"
  instance_type = "t2.micro"
	security_groups = ["${aws_security_group.foo.id}"]
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = ["${aws_subnet.foo.availability_zone}"]
	vpc_zone_identifier = ["${aws_subnet.foo.id}"]
  max_size = 2
  min_size = 2
  health_check_grace_period = 300
  health_check_type = "ELB"
  wait_for_elb_capacity = 2
  force_delete = true

  launch_configuration = "${aws_launch_configuration.foobar.name}"
  load_balancers = ["${aws_elb.bar.name}"]
}
`

const testAccAWSAutoScalingGroupConfigWithAZ = `
resource "aws_vpc" "default" {
  cidr_block = "10.0.0.0/16"
  tags {
     Name = "terraform-test"
  }
}

resource "aws_subnet" "main" {
  vpc_id = "${aws_vpc.default.id}"
  cidr_block = "10.0.1.0/24"
  availability_zone = "us-west-2a"
  tags {
     Name = "terraform-test"
  }
}

resource "aws_launch_configuration" "foobar" {
  image_id = "ami-b5b3fc85"
  instance_type = "t2.micro"
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = [
	  "us-west-2a"
  ]
  desired_capacity = 0
  max_size = 0
  min_size = 0
  launch_configuration = "${aws_launch_configuration.foobar.name}"
}
`

const testAccAWSAutoScalingGroupConfigWithVPCIdent = `
resource "aws_vpc" "default" {
  cidr_block = "10.0.0.0/16"
  tags {
     Name = "terraform-test"
  }
}

resource "aws_subnet" "main" {
  vpc_id = "${aws_vpc.default.id}"
  cidr_block = "10.0.1.0/24"
  availability_zone = "us-west-2a"
  tags {
     Name = "terraform-test"
  }
}

resource "aws_launch_configuration" "foobar" {
  image_id = "ami-b5b3fc85"
  instance_type = "t2.micro"
}

resource "aws_autoscaling_group" "bar" {
  vpc_zone_identifier = [
    "${aws_subnet.main.id}",
  ]
  desired_capacity = 0
  max_size = 0
  min_size = 0
  launch_configuration = "${aws_launch_configuration.foobar.name}"
}
`

func testAccAWSAutoScalingGroupConfig_withPlacementGroup(name string) string {
	return fmt.Sprintf(`
resource "aws_launch_configuration" "foobar" {
  image_id = "ami-21f78e11"
  instance_type = "c3.large"
}

resource "aws_placement_group" "test" {
  name = "%s"
  strategy = "cluster"
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = ["us-west-2a"]
  name = "%s"
  max_size = 1
  min_size = 1
  health_check_grace_period = 300
  health_check_type = "ELB"
  desired_capacity = 1
  force_delete = true
  termination_policies = ["OldestInstance","ClosestToNextInstanceHour"]
  placement_group = "${aws_placement_group.test.name}"

  launch_configuration = "${aws_launch_configuration.foobar.name}"

  tag {
    key = "Foo"
    value = "foo-bar"
    propagate_at_launch = true
  }
}
`, name, name)
}

const testAccAWSAutoscalingMetricsCollectionConfig_allMetricsCollected = `
resource "aws_launch_configuration" "foobar" {
  image_id = "ami-21f78e11"
  instance_type = "t1.micro"
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = ["us-west-2a"]
  max_size = 1
  min_size = 0
  health_check_grace_period = 300
  health_check_type = "EC2"
  desired_capacity = 0
  force_delete = true
  termination_policies = ["OldestInstance","ClosestToNextInstanceHour"]
  launch_configuration = "${aws_launch_configuration.foobar.name}"
  enabled_metrics = ["GroupTotalInstances",
  	     "GroupPendingInstances",
  	     "GroupTerminatingInstances",
  	     "GroupDesiredCapacity",
  	     "GroupInServiceInstances",
  	     "GroupMinSize",
  	     "GroupMaxSize"
  ]
  metrics_granularity = "1Minute"
}
`

const testAccAWSAutoscalingMetricsCollectionConfig_updatingMetricsCollected = `
resource "aws_launch_configuration" "foobar" {
  image_id = "ami-21f78e11"
  instance_type = "t1.micro"
}

resource "aws_autoscaling_group" "bar" {
  availability_zones = ["us-west-2a"]
  max_size = 1
  min_size = 0
  health_check_grace_period = 300
  health_check_type = "EC2"
  desired_capacity = 0
  force_delete = true
  termination_policies = ["OldestInstance","ClosestToNextInstanceHour"]
  launch_configuration = "${aws_launch_configuration.foobar.name}"
  enabled_metrics = ["GroupTotalInstances",
  	     "GroupPendingInstances",
  	     "GroupTerminatingInstances",
  	     "GroupDesiredCapacity",
  	     "GroupMaxSize"
  ]
  metrics_granularity = "1Minute"
}
`

const testAccAWSAutoScalingGroupConfig_ALB_TargetGroup_pre = `
provider "aws" {
  region = "us-west-2"
}

resource "aws_vpc" "default" {
  cidr_block = "10.0.0.0/16"

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}

resource "aws_alb_target_group" "test" {
  name     = "tf-example-alb-tg"
  port     = 80
  protocol = "HTTP"
  vpc_id   = "${aws_vpc.default.id}"
}

resource "aws_subnet" "main" {
  vpc_id            = "${aws_vpc.default.id}"
  cidr_block        = "10.0.1.0/24"
  availability_zone = "us-west-2a"

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}

resource "aws_subnet" "alt" {
  vpc_id            = "${aws_vpc.default.id}"
  cidr_block        = "10.0.2.0/24"
  availability_zone = "us-west-2b"

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}

resource "aws_launch_configuration" "foobar" {
  # Golang-base from cts-hashi aws account, shared with tf testing account
  image_id          = "ami-1817d178"
  instance_type     = "t2.micro"
  enable_monitoring = false
}

resource "aws_autoscaling_group" "bar" {
  vpc_zone_identifier = [
    "${aws_subnet.main.id}",
    "${aws_subnet.alt.id}",
  ]

  max_size                  = 2
  min_size                  = 0
  health_check_grace_period = 300
  health_check_type         = "ELB"
  desired_capacity          = 0
  force_delete              = true
  termination_policies      = ["OldestInstance"]
  launch_configuration      = "${aws_launch_configuration.foobar.name}"

}

resource "aws_security_group" "tf_test_self" {
  name        = "tf_test_alb_asg"
  description = "tf_test_alb_asg"
  vpc_id      = "${aws_vpc.default.id}"

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}
`

const testAccAWSAutoScalingGroupConfig_ALB_TargetGroup_post = `
provider "aws" {
  region = "us-west-2"
}

resource "aws_vpc" "default" {
  cidr_block = "10.0.0.0/16"

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}

resource "aws_alb_target_group" "test" {
  name     = "tf-example-alb-tg"
  port     = 80
  protocol = "HTTP"
  vpc_id   = "${aws_vpc.default.id}"
}

resource "aws_subnet" "main" {
  vpc_id            = "${aws_vpc.default.id}"
  cidr_block        = "10.0.1.0/24"
  availability_zone = "us-west-2a"

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}

resource "aws_subnet" "alt" {
  vpc_id            = "${aws_vpc.default.id}"
  cidr_block        = "10.0.2.0/24"
  availability_zone = "us-west-2b"

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}

resource "aws_launch_configuration" "foobar" {
  # Golang-base from cts-hashi aws account, shared with tf testing account
  image_id          = "ami-1817d178"
  instance_type     = "t2.micro"
  enable_monitoring = false
}

resource "aws_autoscaling_group" "bar" {
  vpc_zone_identifier = [
    "${aws_subnet.main.id}",
    "${aws_subnet.alt.id}",
  ]

	target_group_arns = ["${aws_alb_target_group.test.arn}"]

  max_size                  = 2
  min_size                  = 0
  health_check_grace_period = 300
  health_check_type         = "ELB"
  desired_capacity          = 0
  force_delete              = true
  termination_policies      = ["OldestInstance"]
  launch_configuration      = "${aws_launch_configuration.foobar.name}"

}

resource "aws_security_group" "tf_test_self" {
  name        = "tf_test_alb_asg"
  description = "tf_test_alb_asg"
  vpc_id      = "${aws_vpc.default.id}"

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}
`

const testAccAWSAutoScalingGroupConfig_ALB_TargetGroup_post_duo = `
provider "aws" {
  region = "us-west-2"
}

resource "aws_vpc" "default" {
  cidr_block = "10.0.0.0/16"

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}

resource "aws_alb_target_group" "test" {
  name     = "tf-example-alb-tg"
  port     = 80
  protocol = "HTTP"
  vpc_id   = "${aws_vpc.default.id}"
}

resource "aws_alb_target_group" "test_more" {
  name     = "tf-example-alb-tg-more"
  port     = 80
  protocol = "HTTP"
  vpc_id   = "${aws_vpc.default.id}"
}

resource "aws_subnet" "main" {
  vpc_id            = "${aws_vpc.default.id}"
  cidr_block        = "10.0.1.0/24"
  availability_zone = "us-west-2a"

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}

resource "aws_subnet" "alt" {
  vpc_id            = "${aws_vpc.default.id}"
  cidr_block        = "10.0.2.0/24"
  availability_zone = "us-west-2b"

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}

resource "aws_launch_configuration" "foobar" {
  # Golang-base from cts-hashi aws account, shared with tf testing account
  image_id          = "ami-1817d178"
  instance_type     = "t2.micro"
  enable_monitoring = false
}

resource "aws_autoscaling_group" "bar" {
  vpc_zone_identifier = [
    "${aws_subnet.main.id}",
    "${aws_subnet.alt.id}",
  ]

	target_group_arns = [
		"${aws_alb_target_group.test.arn}",
		"${aws_alb_target_group.test_more.arn}",
	]

  max_size                  = 2
  min_size                  = 0
  health_check_grace_period = 300
  health_check_type         = "ELB"
  desired_capacity          = 0
  force_delete              = true
  termination_policies      = ["OldestInstance"]
  launch_configuration      = "${aws_launch_configuration.foobar.name}"

}

resource "aws_security_group" "tf_test_self" {
  name        = "tf_test_alb_asg"
  description = "tf_test_alb_asg"
  vpc_id      = "${aws_vpc.default.id}"

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags {
    Name = "testAccAWSAutoScalingGroupConfig_ALB_TargetGroup"
  }
}
`
