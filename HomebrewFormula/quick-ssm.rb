class QuickSsm < Formula
  desc "Quickly start AWS SSM Session Manager sessions to EC2 instances"
  homepage "https://github.com/bevelwork/quick_ssm"
  url "https://github.com/bevelwork/quick_ssm/archive/refs/tags/1.15.20250911.tar.gz"
  sha256 "d5558cd419c8d46bdc958064cb97f963d1ea793866414c025906ec15033512ed"
  license "APACHE-2.0"

  head "https://github.com/bevelwork/quick_ssm.git", branch: "main"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(output: bin/"quick-ssm"), "."
  end

  test do
    help_output = shell_output("#{bin}/quick-ssm -h")
    assert_match "quick_ssm", help_output
  end
end


