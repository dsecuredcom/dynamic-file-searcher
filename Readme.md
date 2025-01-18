# Dynamic File Searcher

## Overview

Dynamic File Searcher is an advanced, Go-based CLI tool designed for intelligent and deep web crawling. Its unique
strength lies in its ability to dynamically generate and explore paths based on the target hosts, allowing for much
deeper and more comprehensive scans than traditional tools. This tool is part of <a href="https://www.dsecured.com/en/argos-security-platform">DSecured's eASM Argos</a> since several
years and still generates value for our customers.

### Key Differentiators

- Dynamic path generation based on host structure for deeper, more intelligent scans
- Optional base paths for advanced URL generation
- Flexible word separation options for more targeted searches

While powerful alternatives like nuclei exist, Dynamic File Searcher offers easier handling and more flexibility in path
generation compared to static, template-based approaches.

### Examples of Use Cases

Imagine this being your input data:

- Domain: vendorgo.abc.targetdomain.com
- Paths: env
- Markers: "activeProfiles"

The tool will generate paths like:

- https://vendorgo.abc.targetdomain.com/env
- https://vendorgo.abc.targetdomain.com/vendorgo/env
- https://vendorgo.abc.targetdomain.com/vendorgo-qa/env
- ... and many more

If you add base-paths like "admin" to the mix, the tool will generate even more paths:

- https://vendorgo.abc.targetdomain.com/admin/env
- https://vendorgo.abc.targetdomain.com/admin/vendorgo/env
- https://vendorgo.abc.targetdomain.com/admin/vendorgo-qa/env
- ... and many more

If you know what you are doing, this tool can be a powerful ally in your arsenal for finding issues in web applications
that common web application scanners will certainly miss.

## Features

- Intelligent path generation based on host structure
- Multi-domain or single-domain scanning
- Optional base paths for additional URL generation
- Concurrent requests for high-speed processing
- Content-based file detection using customizable markers
- Large file detection with configurable size thresholds
- Partial content scanning for efficient marker detection in large files
- HTTP status code filtering for focused results
- Custom HTTP header support for advanced probing
- Skipping certain domains when WAF is detected
- Proxy support for anonymous scanning
- Verbose mode for detailed output and analysis

## Installation

### Prerequisites

- Go 1.19 or higher

### Compilation

1. Clone the repository:
   ```
   git clone https://github.com/dsecuredcom/dynamic-file-searcher.git
   cd dynamic-file-searcher
   ```

2. Build the binary:
   ```
   go build -o dynamic_file_searcher
   ```

## Usage

Basic usage:

```
./dynamic_file_searcher -domain <single_domain> -paths <paths_file> [-markers <markers_file>]
```

or

```
./dynamic_file_searcher -domains <domains_file> -paths <paths_file> [-markers <markers_file>]
```

### Command-line Options

- `-domains`: File containing a list of domains to scan (one per line)
- `-domain`: Single domain to scan (alternative to `-domains`)
- `-paths`: File containing a list of paths to check on each domain (required)
- `-markers`: File containing a list of content markers to search for (optional)
- `-base-paths`: File containing list of base paths for additional URL generation (optional) (e.g., "..;/" - it should
  be one per line and end with "/")
- `-concurrency`: Number of concurrent requests (default: 10)
- `-timeout`: Timeout for each request (default: 12s)
- `-verbose`: Enable verbose output
- `-headers`: Extra headers to add to each request (format: 'Header1:Value1,Header2:Value2')
- `-proxy`: Proxy URL (e.g., http://127.0.0.1:8080)
- `-max-content-read`: Maximum size of content to read for marker checking, in bytes (default: 5242880)
- `-force-http`: Force HTTP (instead of HTTPS) requests (default: false)
- `-use-fasthttp`: Use fasthttp instead of net/http (default: false)
- `-host-depth`: How many sub-subdomains to use for path generation (e.g., 2 = test1-abc & test2 [based on test1-abc.test2.test3.example.com])
- `-dont-generate-paths`: Don't generate paths based on host structure (default: false)
- `-dont-append-envs`: Prevent appending environment variables to requests (-qa, ...) (default: false)
- `-append-bypasses-to-words`: Append bypasses to words (admin -> admin; -> admin..;) (default: false)
- `-min-content-size`: Minimum file size to consider, in bytes (default: 0)
- `-http-statuses`: HTTP status code to filter (default: all)
- `-content-types`: Content type to filter(csv allowed, e.g. json,octet)
- `-disallowed-content-types`: Content-Type header value to filter out (csv allowed, e.g. json,octet)
- `-disallowed-content-strings`: Content-Type header value to filter out (csv allowed, e.g. '<html>,<body>')
- `-env-append-words`: Comma-separated list of environment words to append (e.g., dev,prod,api). If not specified, defaults to: prod,qa,dev,test,uat,stg,stage,sit,api

### Examples

1. Scan a single domain:
   ```
   ./dynamic_file_searcher -domain example.com -paths paths.txt -markers markers.txt
   ```

2. Scan multiple domains from a file:
   ```
   ./dynamic_file_searcher -domains domains.txt -paths paths.txt -markers markers.txt
   ```

3. Use base paths for additional URL generation:
   ```
   ./dynamic_file_searcher -domain example.com -paths paths.txt -markers markers.txt -base-paths base_paths.txt
   ```

4. Scan for large files (>5MB) with content type JSON:
   ```
   ./dynamic_file_searcher -domains domains.txt -paths paths.txt -min-content-size 5000000 -content-types json -http-statuses 200,206
   ```

5. Targeted scan through a proxy with custom headers:
   ```
   ./dynamic_file_searcher -domain example.com -paths paths.txt -markers markers.txt -proxy http://127.0.0.1:8080-headers "User-Agent:CustomBot/1.0"
   ```

6. Verbose output with custom timeout:
   ```
   ./dynamic_file_searcher -domain example.com -paths paths.txt -markers markers.txt -verbose -timeout 30s
   ```

7. Scan only root paths without generating additional paths:
   ```
   ./dynamic_file_searcher -domain example.com -paths paths.txt -markers markers.txt -dont-generate-paths
   ```

## Understanding the flags

There are basically some very important flags that you should understand before using the tool. These flags are:

- `-host-depth`
- `-dont-generate-paths`
- `-dont-append-envs`
- `-append-bypasses-to-words`
- `-env-append-words`

Given the following host structure: `housetodo.some-word.thisthat.example.com`

### host-depth

This flag is used to determine how many sub-subdomains to use for path generation. For example, if `-host-depth` is set
to 2, the tool will generate paths based on `housetodo.some-word`. If `-host-depth` is set to 1, the tool will generate
paths based on `housetodo` only.

### dont-generate-paths

This will simply prevent the tool from generating paths based on the host structure. If this flag is enabled, the tool
will only use the paths provided in the `-paths` file as well as in the `-base-paths` file.

### dont-append-envs

This tool tries to generate sane value for relevant words. In our example one of those words would be `housetodo`. If
this flag is enabled, the tool will not append environment variables to the requests. For example, if the tool
detects `housetodo` as a word, it will not append `-qa`, `-dev`, `-prod`, etc. to the word.

### append-bypasses-to-words

This flag is used to append bypasses to words. For example, if the tool detects `admin` as a word, it will
append `admin;` and `admin..;` etc. to the word. This is useful for bypassing filters.

### env-append-words
This flag allows you to customize the list of environment words that will be appended to relevant words during path generation. 
By default, the tool uses a predefined list: `prod,qa,dev,test,uat,stg,stage,sit,api`. 
You can override this with your own comma-separated list of words.

For example:

`./dynamic_file_searcher -domain example.com -paths paths.txt -env-append-words "development,production,staging,beta"`

This would generate paths like:
- /housetodo-development
- /housetodo-production
- /housetodo-staging
- /housetodo-beta

Note that this flag only has an effect if `-dont-append-envs` is not set. 
When `-dont-append-envs` is true, no environment words will be appended regardless of the `-env-append-words` value.

## How It Works

1. The tool reads the domain(s) from either the `-domain` flag or the `-domains` file.
2. It reads the list of paths from the specified `-paths` file.
3. If provided, it reads additional base paths from the `-base-paths` file.
4. It analyzes each domain to extract meaningful components (subdomains, main domain, etc.).
5. Using these components and the provided paths (and base paths if available), it dynamically generates a comprehensive
   set of URLs to scan.
6. Concurrent workers send HTTP GET requests to these URLs.
7. For each response:
    - The tool reads up to `max-content-read` bytes for marker checking.
    - It determines the full file size by reading (and discarding) the remaining content.
    - The response is analyzed based on:
        * Presence of specified content markers in the read portion (if markers are provided)
        * OR -->
        * Total file size (compared against `min-content-size`)
        * Content types (if specified) + Disallowed content types (if specified)
        * Disallowed content strings (if specified)
        * HTTP status code
        * Important: These rules are not applied to marker based checks
8. Results are reported in real-time, with a progress bar indicating overall completion.

This approach allows for efficient scanning of both small and large files, balancing thorough marker checking with
memory-efficient handling of large files.

## Large File Handling

The tool efficiently handles large files and octet streams by:

- Reading a configurable portion of the file for marker checking
- Determining the full file size without loading the entire file into memory
- Reporting both on file size and marker presence, even for partially read files

This allows for effective scanning of large files without running into memory issues.

It is recommended to use a big timeout to allow the tool to read large files. The default timeout is 10 seconds.

## Security Considerations

- Always ensure you have explicit permission to scan the target domains.
- Use the proxy option for anonymity when necessary.
- Be mindful of the load your scans might place on target servers.
- Respect robots.txt files and website terms of service.

## Limitations

- There's no built-in rate limiting (use the concurrency option to control request rate).
- Very large scale scans might require significant bandwidth and processing power. It is recommended to separate the
  input files and run multiple instances of the tool on different machines.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.


## Disclaimer

This tool is for educational and authorized testing purposes only. Misuse of this tool may be illegal. The authors are
not responsible for any unauthorized use or damage caused by this tool.