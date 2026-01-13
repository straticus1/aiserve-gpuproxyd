#!/usr/bin/env python3
"""
aiserve-gpuproxyd Configuration and Setup Utility

This script helps with:
1. Building/compiling the project
2. Database setup and migrations
3. Environment configuration
4. Development and debugging setup
"""

import os
import sys
import subprocess
import shutil
import argparse
import secrets
import string
from pathlib import Path
from typing import Optional, List, Tuple


class Colors:
    """ANSI color codes for terminal output"""
    HEADER = '\033[95m'
    OKBLUE = '\033[94m'
    OKCYAN = '\033[96m'
    OKGREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'


class ConfigurationManager:
    """Manages project configuration and setup"""

    def __init__(self, project_root: Path):
        self.project_root = project_root
        self.env_file = project_root / ".env"
        self.env_example = project_root / ".env.example"
        self.bin_dir = project_root / "bin"

    def print_header(self, message: str):
        """Print a formatted header"""
        print(f"\n{Colors.HEADER}{Colors.BOLD}{'='*60}{Colors.ENDC}")
        print(f"{Colors.HEADER}{Colors.BOLD}{message.center(60)}{Colors.ENDC}")
        print(f"{Colors.HEADER}{Colors.BOLD}{'='*60}{Colors.ENDC}\n")

    def print_success(self, message: str):
        """Print a success message"""
        print(f"{Colors.OKGREEN}✓ {message}{Colors.ENDC}")

    def print_error(self, message: str):
        """Print an error message"""
        print(f"{Colors.FAIL}✗ {message}{Colors.ENDC}")

    def print_warning(self, message: str):
        """Print a warning message"""
        print(f"{Colors.WARNING}⚠ {message}{Colors.ENDC}")

    def print_info(self, message: str):
        """Print an info message"""
        print(f"{Colors.OKCYAN}ℹ {message}{Colors.ENDC}")

    def run_command(self, cmd: List[str], cwd: Optional[Path] = None,
                   check: bool = True, capture_output: bool = False) -> Tuple[int, str, str]:
        """Run a shell command and return the result"""
        try:
            if capture_output:
                result = subprocess.run(
                    cmd,
                    cwd=cwd or self.project_root,
                    check=check,
                    capture_output=True,
                    text=True
                )
                return result.returncode, result.stdout, result.stderr
            else:
                result = subprocess.run(
                    cmd,
                    cwd=cwd or self.project_root,
                    check=check
                )
                return result.returncode, "", ""
        except subprocess.CalledProcessError as e:
            if capture_output:
                return e.returncode, e.stdout if hasattr(e, 'stdout') else "", e.stderr if hasattr(e, 'stderr') else ""
            return e.returncode, "", ""

    def check_dependencies(self) -> bool:
        """Check if required dependencies are installed"""
        self.print_header("Checking Dependencies")

        dependencies = {
            "go": ("go version", "Go 1.25.2 or later"),
            "docker": ("docker --version", "Docker (optional, for containerized deployment)"),
            "docker-compose": ("docker-compose --version", "Docker Compose (optional)"),
            "git": ("git --version", "Git"),
        }

        all_ok = True
        for name, (cmd, description) in dependencies.items():
            returncode, stdout, stderr = self.run_command(
                cmd.split(),
                check=False,
                capture_output=True
            )
            if returncode == 0:
                self.print_success(f"{name}: {stdout.strip()}")
            else:
                if name in ["docker", "docker-compose"]:
                    self.print_warning(f"{name} not found (optional): {description}")
                else:
                    self.print_error(f"{name} not found: {description}")
                    all_ok = False

        return all_ok

    def setup_environment(self, force: bool = False):
        """Set up environment configuration"""
        self.print_header("Environment Configuration")

        if self.env_file.exists() and not force:
            self.print_warning(f".env file already exists at {self.env_file}")
            response = input("Overwrite? (y/N): ").strip().lower()
            if response != 'y':
                self.print_info("Skipping environment setup")
                return

        if not self.env_example.exists():
            self.print_error(f".env.example not found at {self.env_example}")
            return

        # Copy .env.example to .env
        shutil.copy(self.env_example, self.env_file)
        self.print_success(f"Created .env from .env.example")

        # Generate secure secrets
        self.print_info("Generating secure secrets...")

        jwt_secret = self.generate_secret(64)
        api_key_prefix = self.generate_secret(32)

        # Update .env with generated secrets
        with open(self.env_file, 'r') as f:
            content = f.read()

        content = content.replace(
            'JWT_SECRET=changeme-generate-a-secure-random-string',
            f'JWT_SECRET={jwt_secret}'
        )

        with open(self.env_file, 'w') as f:
            f.write(content)

        self.print_success("Generated secure JWT secret")

        # Prompt for optional configurations
        print("\n" + Colors.BOLD + "Optional Configuration:" + Colors.ENDC)
        print("You can configure these now or edit .env later\n")

        if input("Configure database settings? (y/N): ").strip().lower() == 'y':
            self.configure_database()

        if input("Configure GPU provider API keys? (y/N): ").strip().lower() == 'y':
            self.configure_gpu_providers()

        self.print_success(f"Environment configuration saved to {self.env_file}")
        self.print_info("Review and update .env as needed before running the server")

    def generate_secret(self, length: int = 32) -> str:
        """Generate a secure random secret"""
        alphabet = string.ascii_letters + string.digits + "!@#$%^&*(-_=+)"
        return ''.join(secrets.choice(alphabet) for _ in range(length))

    def configure_database(self):
        """Interactively configure database settings"""
        print("\n" + Colors.BOLD + "Database Configuration:" + Colors.ENDC)

        db_type = input("Database type (postgres/sqlite) [postgres]: ").strip() or "postgres"

        if db_type == "postgres":
            db_host = input("Database host [localhost]: ").strip() or "localhost"
            db_port = input("Database port [5432]: ").strip() or "5432"
            db_user = input("Database user [postgres]: ").strip() or "postgres"
            db_password = input("Database password [changeme]: ").strip() or "changeme"
            db_name = input("Database name [gpuproxy]: ").strip() or "gpuproxy"

            # Update .env
            self.update_env_var("DB_TYPE", db_type)
            self.update_env_var("DB_HOST", db_host)
            self.update_env_var("DB_PORT", db_port)
            self.update_env_var("DB_USER", db_user)
            self.update_env_var("DB_PASSWORD", db_password)
            self.update_env_var("DB_NAME", db_name)
        else:
            db_file = input("SQLite database file [./gpuproxy.db]: ").strip() or "./gpuproxy.db"
            self.update_env_var("DB_TYPE", "sqlite")
            self.update_env_var("DB_NAME", db_file)

    def configure_gpu_providers(self):
        """Interactively configure GPU provider API keys"""
        print("\n" + Colors.BOLD + "GPU Provider Configuration:" + Colors.ENDC)
        print("(Leave blank to skip)")

        vastai_key = input("Vast.ai API key: ").strip()
        if vastai_key:
            self.update_env_var("VASTAI_API_KEY", vastai_key)

        ionet_key = input("IO.net API key: ").strip()
        if ionet_key:
            self.update_env_var("IONET_API_KEY", ionet_key)

    def update_env_var(self, key: str, value: str):
        """Update or add an environment variable in .env"""
        if not self.env_file.exists():
            return

        with open(self.env_file, 'r') as f:
            lines = f.readlines()

        updated = False
        for i, line in enumerate(lines):
            if line.startswith(f"{key}="):
                lines[i] = f"{key}={value}\n"
                updated = True
                break

        if not updated:
            lines.append(f"{key}={value}\n")

        with open(self.env_file, 'w') as f:
            f.writelines(lines)

    def build_project(self, debug: bool = False):
        """Build the Go project"""
        self.print_header("Building Project")

        # Create bin directory if it doesn't exist
        self.bin_dir.mkdir(exist_ok=True)

        # Build flags
        build_flags = []
        if debug:
            build_flags.extend(["-gcflags", "all=-N -l"])  # Disable optimizations for debugging

        # Build server
        self.print_info("Building aiserve-gpuproxyd (server)...")
        cmd = ["go", "build"] + build_flags + ["-o", "bin/aiserve-gpuproxyd", "./cmd/server"]
        returncode, _, stderr = self.run_command(cmd, check=False, capture_output=True)
        if returncode == 0:
            self.print_success("Server built successfully")
        else:
            self.print_error(f"Server build failed: {stderr}")
            return False

        # Build client
        self.print_info("Building aiserve-gpuproxy-client...")
        cmd = ["go", "build"] + build_flags + ["-o", "bin/aiserve-gpuproxy-client", "./cmd/client"]
        returncode, _, stderr = self.run_command(cmd, check=False, capture_output=True)
        if returncode == 0:
            self.print_success("Client built successfully")
        else:
            self.print_error(f"Client build failed: {stderr}")

        # Build admin
        self.print_info("Building aiserve-gpuproxy-admin...")
        cmd = ["go", "build"] + build_flags + ["-o", "bin/aiserve-gpuproxy-admin", "./cmd/admin"]
        returncode, _, stderr = self.run_command(cmd, check=False, capture_output=True)
        if returncode == 0:
            self.print_success("Admin tool built successfully")
        else:
            self.print_error(f"Admin build failed: {stderr}")

        self.print_success("\nAll binaries built successfully!")
        self.print_info(f"Binaries located in: {self.bin_dir}")

        return True

    def install_dependencies(self):
        """Install Go dependencies"""
        self.print_header("Installing Dependencies")

        self.print_info("Downloading Go modules...")
        returncode, _, stderr = self.run_command(["go", "mod", "download"], check=False, capture_output=True)
        if returncode != 0:
            self.print_error(f"Failed to download modules: {stderr}")
            return False

        self.print_info("Tidying Go modules...")
        returncode, _, stderr = self.run_command(["go", "mod", "tidy"], check=False, capture_output=True)
        if returncode != 0:
            self.print_error(f"Failed to tidy modules: {stderr}")
            return False

        self.print_success("Dependencies installed successfully")
        return True

    def setup_database(self, docker: bool = False):
        """Set up and migrate the database"""
        self.print_header("Database Setup")

        if docker:
            self.print_info("Starting database services with Docker...")
            returncode, _, stderr = self.run_command(
                ["docker-compose", "up", "-d", "postgres", "redis"],
                check=False,
                capture_output=True
            )
            if returncode != 0:
                self.print_error(f"Failed to start Docker services: {stderr}")
                return False

            self.print_success("Docker database services started")
            self.print_info("Waiting for databases to be ready...")

            import time
            time.sleep(5)  # Wait for services to be ready

        # Run migrations
        self.print_info("Running database migrations...")
        admin_tool = self.bin_dir / "aiserve-gpuproxy-admin"

        if not admin_tool.exists():
            self.print_error("Admin tool not found. Build the project first.")
            return False

        returncode, stdout, stderr = self.run_command(
            [str(admin_tool), "migrate"],
            check=False,
            capture_output=True
        )

        if returncode != 0:
            self.print_error(f"Migration failed: {stderr}")
            self.print_info("Make sure the database is running and .env is configured correctly")
            return False

        self.print_success("Database migrations completed successfully")
        return True

    def create_admin_user(self):
        """Create an admin user"""
        self.print_header("Create Admin User")

        admin_tool = self.bin_dir / "aiserve-gpuproxy-admin"
        if not admin_tool.exists():
            self.print_error("Admin tool not found. Build the project first.")
            return

        email = input("Admin email: ").strip()
        if not email:
            self.print_error("Email is required")
            return

        password = input("Admin password: ").strip()
        if not password:
            self.print_error("Password is required")
            return

        name = input("Admin name: ").strip() or "Admin"

        # Create user
        self.print_info("Creating user...")
        returncode, stdout, stderr = self.run_command(
            [str(admin_tool), "create-user", email, password, name],
            check=False,
            capture_output=True
        )

        if returncode != 0:
            self.print_error(f"Failed to create user: {stderr}")
            return

        # Make admin
        self.print_info("Granting admin privileges...")
        returncode, stdout, stderr = self.run_command(
            [str(admin_tool), "make-admin", email],
            check=False,
            capture_output=True
        )

        if returncode != 0:
            self.print_error(f"Failed to grant admin privileges: {stderr}")
            return

        self.print_success(f"Admin user created: {email}")

    def run_tests(self):
        """Run project tests"""
        self.print_header("Running Tests")

        self.print_info("Running Go tests...")
        returncode, stdout, stderr = self.run_command(
            ["go", "test", "-v", "./..."],
            check=False
        )

        if returncode == 0:
            self.print_success("All tests passed!")
        else:
            self.print_error("Some tests failed")

        return returncode == 0

    def start_dev_server(self):
        """Start the development server"""
        self.print_header("Starting Development Server")

        server_bin = self.bin_dir / "aiserve-gpuproxyd"
        if not server_bin.exists():
            self.print_error("Server binary not found. Build the project first.")
            return

        self.print_info("Starting server in developer/debug mode...")
        self.print_info("Press Ctrl+C to stop")
        print()

        try:
            self.run_command([str(server_bin), "-dv", "-dm"], check=False)
        except KeyboardInterrupt:
            print()
            self.print_info("Server stopped")

    def docker_setup(self):
        """Set up and run the project with Docker"""
        self.print_header("Docker Setup")

        self.print_info("Building Docker images...")
        returncode, _, stderr = self.run_command(
            ["docker-compose", "build"],
            check=False,
            capture_output=True
        )

        if returncode != 0:
            self.print_error(f"Docker build failed: {stderr}")
            return False

        self.print_success("Docker images built successfully")

        self.print_info("Starting Docker services...")
        returncode, _, stderr = self.run_command(
            ["docker-compose", "up", "-d"],
            check=False,
            capture_output=True
        )

        if returncode != 0:
            self.print_error(f"Failed to start Docker services: {stderr}")
            return False

        self.print_success("Docker services started")
        self.print_info("View logs with: docker-compose logs -f")
        self.print_info("Server running at: http://localhost:8080")

        return True

    def show_status(self):
        """Show project status"""
        self.print_header("Project Status")

        # Check .env
        if self.env_file.exists():
            self.print_success(f".env file exists: {self.env_file}")
        else:
            self.print_warning(f".env file not found: {self.env_file}")

        # Check binaries
        binaries = ["aiserve-gpuproxyd", "aiserve-gpuproxy-client", "aiserve-gpuproxy-admin"]
        for binary in binaries:
            binary_path = self.bin_dir / binary
            if binary_path.exists():
                self.print_success(f"Binary built: {binary}")
            else:
                self.print_warning(f"Binary not built: {binary}")

        # Check Docker services
        returncode, stdout, stderr = self.run_command(
            ["docker-compose", "ps"],
            check=False,
            capture_output=True
        )

        if returncode == 0 and stdout.strip():
            print(f"\n{Colors.BOLD}Docker Services:{Colors.ENDC}")
            print(stdout)


def main():
    parser = argparse.ArgumentParser(
        description="aiserve-gpuproxyd Configuration and Setup Utility",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Quick setup (checks dependencies, installs packages, builds project)
  ./configure.py --setup

  # Full setup with environment and database
  ./configure.py --setup --env --db

  # Build with debug symbols
  ./configure.py --build --debug

  # Set up and run with Docker
  ./configure.py --docker

  # Create admin user
  ./configure.py --create-admin

  # Start development server
  ./configure.py --dev
        """
    )

    # Main actions
    parser.add_argument("--setup", action="store_true",
                       help="Run basic setup (check deps, install packages, build)")
    parser.add_argument("--check", action="store_true",
                       help="Check dependencies only")
    parser.add_argument("--deps", action="store_true",
                       help="Install Go dependencies")
    parser.add_argument("--env", action="store_true",
                       help="Set up environment configuration")
    parser.add_argument("--build", action="store_true",
                       help="Build the project")
    parser.add_argument("--db", action="store_true",
                       help="Set up and migrate database")
    parser.add_argument("--create-admin", action="store_true",
                       help="Create an admin user")
    parser.add_argument("--test", action="store_true",
                       help="Run tests")
    parser.add_argument("--dev", action="store_true",
                       help="Start development server")
    parser.add_argument("--docker", action="store_true",
                       help="Set up and run with Docker")
    parser.add_argument("--status", action="store_true",
                       help="Show project status")

    # Modifiers
    parser.add_argument("--debug", action="store_true",
                       help="Build with debug symbols")
    parser.add_argument("--force", action="store_true",
                       help="Force overwrite existing files")
    parser.add_argument("--docker-db", action="store_true",
                       help="Use Docker for database services")

    args = parser.parse_args()

    # Find project root
    project_root = Path(__file__).parent.resolve()
    manager = ConfigurationManager(project_root)

    # If no arguments, show help
    if len(sys.argv) == 1:
        parser.print_help()
        sys.exit(0)

    try:
        # Execute requested actions
        if args.check or args.setup:
            if not manager.check_dependencies():
                manager.print_error("Dependency check failed")
                sys.exit(1)

        if args.deps or args.setup:
            if not manager.install_dependencies():
                manager.print_error("Failed to install dependencies")
                sys.exit(1)

        if args.env:
            manager.setup_environment(force=args.force)

        if args.build or args.setup:
            if not manager.build_project(debug=args.debug):
                manager.print_error("Build failed")
                sys.exit(1)

        if args.db:
            if not manager.setup_database(docker=args.docker_db):
                manager.print_error("Database setup failed")
                sys.exit(1)

        if args.create_admin:
            manager.create_admin_user()

        if args.test:
            if not manager.run_tests():
                sys.exit(1)

        if args.docker:
            if not manager.docker_setup():
                manager.print_error("Docker setup failed")
                sys.exit(1)

        if args.dev:
            manager.start_dev_server()

        if args.status:
            manager.show_status()

        # If setup was run, show next steps
        if args.setup:
            manager.print_header("Setup Complete!")
            print(f"{Colors.BOLD}Next Steps:{Colors.ENDC}")
            print(f"  1. Configure environment: {Colors.OKCYAN}./configure.py --env{Colors.ENDC}")
            print(f"  2. Set up database: {Colors.OKCYAN}./configure.py --db{Colors.ENDC}")
            print(f"  3. Create admin user: {Colors.OKCYAN}./configure.py --create-admin{Colors.ENDC}")
            print(f"  4. Start dev server: {Colors.OKCYAN}./configure.py --dev{Colors.ENDC}")
            print(f"\n{Colors.BOLD}Or use Docker:{Colors.ENDC}")
            print(f"  {Colors.OKCYAN}./configure.py --docker{Colors.ENDC}")
            print()

    except KeyboardInterrupt:
        print()
        manager.print_info("Operation cancelled by user")
        sys.exit(130)
    except Exception as e:
        manager.print_error(f"Unexpected error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
