class QuickSsm < Formula
  desc "Quickly start AWS SSM Session Manager sessions to EC2 instances"
  homepage "https://github.com/bevelwork/quick_ssm"
  url "https://github.com/bevelwork/quick_ssm/archive/refs/tags/v0.0.00000000.tar.gz"
  sha256 "sha256placeholder"
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


