# Homebrew Formula Template for ecs2k8s
# This file shows how to create the formula for the homebrew-ecs2k8s tap
# Save this as: Formula/ecs2k8s.rb in the homebrew-ecs2k8s tap repository

class Ecs2k8s < Formula
  desc "AWS ECS to Kubernetes migration tool"
  homepage "https://github.com/krishnaduttPanchagnula/ecs2k8s"
  url "https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/download/v{VERSION}/ecs2k8s_{VERSION}_darwin_{ARCH}.tar.gz"
  sha256 "{SHA256_HASH}"
  license "Apache-2.0"

  version "{VERSION}"

  # Disable binary detection for this formula
  pour_bottle? false if OS.linux?

  on_macos do
    on_arm64 do
      url "https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/download/v{VERSION}/ecs2k8s_{VERSION}_darwin_arm64.tar.gz"
      sha256 "{SHA256_ARM64}"
    end
    on_intel do
      url "https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/download/v{VERSION}/ecs2k8s_{VERSION}_darwin_amd64.tar.gz"
      sha256 "{SHA256_AMD64}"
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/download/v{VERSION}/ecs2k8s_{VERSION}_linux_amd64.tar.gz"
      sha256 "{SHA256_LINUX_AMD64}"
    end
    on_arm do
      url "https://github.com/krishnaduttPanchagnula/ecs2k8s/releases/download/v{VERSION}/ecs2k8s_{VERSION}_linux_arm64.tar.gz"
      sha256 "{SHA256_LINUX_ARM64}"
    end
  end

  def install
    bin.install "ecs2k8s"
  end

  def post_install
    puts "ecs2k8s has been installed successfully!"
    puts "Run 'ecs2k8s --help' to get started"
  end

  test do
    system "#{bin}/ecs2k8s", "--help"
    assert_match "AWS ECS to Kubernetes", shell_output("#{bin}/ecs2k8s --help")
  end
end
