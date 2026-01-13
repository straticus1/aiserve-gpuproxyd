from setuptools import setup, find_packages

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

setup(
    name="aiserve",
    version="1.0.1",
    author="AfterDark Systems",
    author_email="support@afterdarksys.com",
    description="Official Python SDK for AIServe.Farm GPU Proxy and AI Model Inference Platform",
    long_description=long_description,
    long_description_content_type="text/markdown",
    url="https://github.com/straticus1/aiserve-gpuproxyd",
    project_urls={
        "Bug Tracker": "https://github.com/straticus1/aiserve-gpuproxyd/issues",
        "Documentation": "https://aiserve.farm/docs",
        "Source Code": "https://github.com/straticus1/aiserve-gpuproxyd",
    },
    packages=find_packages(where="src"),
    package_dir={"": "src"},
    classifiers=[
        "Development Status :: 5 - Production/Stable",
        "Intended Audience :: Developers",
        "Topic :: Software Development :: Libraries :: Python Modules",
        "Topic :: Scientific/Engineering :: Artificial Intelligence",
        "License :: OSI Approved :: MIT License",
        "Programming Language :: Python :: 3",
        "Programming Language :: Python :: 3.8",
        "Programming Language :: Python :: 3.9",
        "Programming Language :: Python :: 3.10",
        "Programming Language :: Python :: 3.11",
        "Programming Language :: Python :: 3.12",
    ],
    python_requires=">=3.8",
    install_requires=[
        "requests>=2.28.0",
        "aiohttp>=3.8.0",
    ],
    extras_require={
        "dev": [
            "pytest>=7.0.0",
            "pytest-cov>=4.0.0",
            "pytest-asyncio>=0.21.0",
            "black>=23.0.0",
            "mypy>=1.0.0",
            "types-requests",
        ],
    },
    keywords="aiserve gpu inference ml ai model-serving pytorch onnx tensorflow mcp agent",
)
