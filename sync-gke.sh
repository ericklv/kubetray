#!/usr/bin/env bash

# ==============================================================================
# Script: sync-gke.sh
# Description: Discovers GKE clusters across all GCP projects, prompts for auth
#              if needed, excludes 'sys-*' projects, and registers clusters into
#              kubeconfig avoiding duplicates.
# ==============================================================================

set -uo pipefail

# ANSI color codes for terminal output
readonly COLOR_RESET="\033[0m"
readonly COLOR_BOLD="\033[1m"
readonly COLOR_GREEN="\033[32m"
readonly COLOR_YELLOW="\033[33m"
readonly COLOR_BLUE="\033[34m"
readonly COLOR_CYAN="\033[36m"
readonly COLOR_RED="\033[31m"
readonly COLOR_DIM="\033[2m"

# Configuration variables and flags
DRY_RUN=false
FORCE=false
INTERNAL_IP=false
VERBOSE=false

log_info() {
    echo -e "${COLOR_BLUE}[INFO]${COLOR_RESET} $1"
}

log_success() {
    echo -e "${COLOR_GREEN}[✔]${COLOR_RESET} $1"
}

log_warn() {
    echo -e "${COLOR_YELLOW}[!]${COLOR_RESET} $1"
}

log_error() {
    echo -e "${COLOR_RED}[✖]${COLOR_RESET} $1" >&2
}

log_header() {
    echo -e "\n${COLOR_BOLD}${COLOR_CYAN}=== $1 ===${COLOR_RESET}"
}

show_help() {
    echo -e "${COLOR_BOLD}Usage:${COLOR_RESET} ./sync-gke.sh [OPTIONS]"
    echo ""
    echo -e "${COLOR_BOLD}Description:${COLOR_RESET}"
    echo "  Scans all GCP projects (excluding those with 'sys-' prefix),"
    echo "  finds Google Kubernetes Engine (GKE) clusters, and adds their credentials to"
    echo "  your local kubeconfig without creating duplicate entries."
    echo ""
    echo -e "${COLOR_BOLD}Options:${COLOR_RESET}"
    echo "  -d, --dry-run       Show clusters that would be added without modifying kubeconfig."
    echo "  -f, --force         Force credentials update even if the context already exists."
    echo "  -i, --internal-ip   Use the cluster's internal IP (--internal-ip) for credentials."
    echo "  -v, --verbose       Display detailed execution information."
    echo "  -h, --help          Show this help message and exit."
    echo ""
}

# Parse command-line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -d|--dry-run)
                DRY_RUN=true
                shift
                ;;
            -f|--force)
                FORCE=true
                shift
                ;;
            -i|--internal-ip)
                INTERNAL_IP=true
                shift
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unrecognized option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Validate required CLI dependencies
check_dependencies() {
    local missing_tools=()
    for tool in gcloud kubectl; do
        if ! command -v "$tool" &>/dev/null; then
            missing_tools+=("$tool")
        fi
    done

    if [[ ${#missing_tools[@]} -gt 0 ]]; then
        log_error "Missing required tools in PATH: ${missing_tools[*]}"
        log_info "Please install Google Cloud SDK (gcloud) and kubectl before running this script."
        exit 1
    fi

    # Check for GKE kubectl auth plugin (required by Kubernetes >= 1.26)
    if ! command -v gke-gcloud-auth-plugin &>/dev/null; then
        log_warn "Binary 'gke-gcloud-auth-plugin' was not found in PATH."
        log_info "Kubernetes v1.26+ requires this plugin to authenticate kubectl with GKE."
        log_info "To install it, run:"
        log_info "  - gcloud components install gke-gcloud-auth-plugin"
        log_info "  - OR via package manager: sudo apt-get install google-cloud-cli-gke-gcloud-auth-plugin / dnf / yay"
        echo ""
    fi
}

# Check and interactively establish GCP session if not active
ensure_gcp_session() {
    log_info "Checking for active Google Cloud session..."
    local active_account
    active_account=$(gcloud auth list --filter="status:ACTIVE" --format="value(account)" 2>/dev/null | head -n 1 || true)

    if [[ -z "$active_account" ]]; then
        log_warn "No active GCP session detected in gcloud."
        
        # Prompt user to open browser and authenticate
        read -r -p "$(echo -e "${COLOR_BOLD}Would you like to open your browser to log in with 'gcloud auth login'? [y/N]: ${COLOR_RESET}")" user_response
        
        case "$user_response" in
            [yY]|[yY][eE][sS]|[sS]|[sS][iI])
                log_info "Opening browser to log into GCP..."
                if ! gcloud auth login; then
                    log_error "Authentication with gcloud failed."
                    exit 1
                fi
                # Re-validate active account
                active_account=$(gcloud auth list --filter="status:ACTIVE" --format="value(account)" 2>/dev/null | head -n 1 || true)
                if [[ -z "$active_account" ]]; then
                    log_error "Could not retrieve an active account after login."
                    exit 1
                fi
                log_success "Successfully logged in as: ${COLOR_BOLD}$active_account${COLOR_RESET}"
                ;;
            *)
                log_warn "Operation cancelled by user. An active session is required to proceed."
                exit 0
                ;;
        esac
    else
        log_success "Active session found: ${COLOR_BOLD}$active_account${COLOR_RESET}"
    fi
}

main() {
    parse_arguments "$@"
    check_dependencies
    ensure_gcp_session

    if [[ "$DRY_RUN" == true ]]; then
        log_warn "DRY-RUN MODE ACTIVE: No changes will be written to kubeconfig."
    fi

    log_header "Fetching GCP Project List"
    
    # Retrieve all GCP projects
    local all_projects
    mapfile -t all_projects < <(gcloud projects list --format="value(projectId)" 2>/dev/null || true)

    if [[ ${#all_projects[@]} -eq 0 ]]; then
        log_error "No projects found for current account or insufficient permissions to list projects."
        exit 1
    fi

    log_info "Total projects discovered: ${#all_projects[@]}"

    # Filter out projects starting with 'sys-'
    local valid_projects=()
    local excluded_sys_count=0

    for project in "${all_projects[@]}"; do
        if [[ "$project" =~ ^sys- ]]; then
            ((excluded_sys_count++))
            if [[ "$VERBOSE" == true ]]; then
                echo -e "  ${COLOR_DIM}[Excluded] System project: $project${COLOR_RESET}"
            fi
        else
            valid_projects+=("$project")
        fi
    done

    log_info "Projects excluded ('sys-' prefix): $excluded_sys_count"
    log_info "Valid projects to scan: ${#valid_projects[@]}"

    # Retrieve existing contexts in kubeconfig
    log_header "Reading Existing Contexts from Kubeconfig"
    local existing_contexts
    mapfile -t existing_contexts < <(kubectl config get-contexts -o name 2>/dev/null || true)
    
    # Create associative array for O(1) lookup
    declare -A context_map
    for ctx in "${existing_contexts[@]}"; do
        context_map["$ctx"]=1
    done

    log_info "Current contexts in kubeconfig: ${#existing_contexts[@]}"

    # Summary metrics
    local total_clusters_found=0
    local clusters_added=0
    local clusters_skipped=0
    local projects_with_gke=0
    local projects_with_errors=0

    log_header "Scanning GKE Clusters in Valid Projects"

    local current_idx=0
    local total_valid=${#valid_projects[@]}

    for project in "${valid_projects[@]}"; do
        ((current_idx++))
        echo -e "\n${COLOR_CYAN}[$current_idx/$total_valid]${COLOR_RESET} Scanning project: ${COLOR_BOLD}$project${COLOR_RESET}"

        # Query clusters in the project
        local clusters_output
        if ! clusters_output=$(gcloud container clusters list --project="$project" --format="csv[no-heading](name,location)" 2>/dev/null); then
            log_warn "  Could not query clusters in '$project' (Container API disabled or insufficient permissions)."
            ((projects_with_errors++))
            continue
        fi

        if [[ -z "$clusters_output" ]]; then
            echo -e "  ${COLOR_DIM}No GKE clusters found in this project.${COLOR_RESET}"
            continue
        fi

        ((projects_with_gke++))

        # Process each discovered cluster
        while IFS=',' read -r cluster_name cluster_location; do
            [[ -z "$cluster_name" || -z "$cluster_location" ]] && continue
            ((total_clusters_found++))

            # Standard context naming convention: gke_${PROJECT}_${LOCATION}_${CLUSTER_NAME}
            local expected_context="gke_${project}_${cluster_location}_${cluster_name}"

            if [[ -n "${context_map[$expected_context]:-}" && "$FORCE" == false ]]; then
                echo -e "  ${COLOR_YELLOW}↷ Skipped:${COLOR_RESET} Cluster '${COLOR_BOLD}$cluster_name${COLOR_RESET}' ($cluster_location) already exists in kubeconfig [${expected_context}]"
                ((clusters_skipped++))
            else
                if [[ "$DRY_RUN" == true ]]; then
                    echo -e "  ${COLOR_GREEN}+ [DRY-RUN] Would add:${COLOR_RESET} Cluster '${COLOR_BOLD}$cluster_name${COLOR_RESET}' ($cluster_location)"
                    ((clusters_added++))
                else
                    echo -e "  ${COLOR_GREEN}➔ Fetching credentials:${COLOR_RESET} Cluster '${COLOR_BOLD}$cluster_name${COLOR_RESET}' ($cluster_location)..."
                    
                    local get_creds_cmd=(gcloud container clusters get-credentials "$cluster_name" --project="$project" --location="$cluster_location")
                    if [[ "$INTERNAL_IP" == true ]]; then
                        get_creds_cmd+=(--internal-ip)
                    fi

                    if "${get_creds_cmd[@]}" &>/dev/null; then
                        log_success "  Context registered: $expected_context"
                        context_map["$expected_context"]=1
                        ((clusters_added++))
                    else
                        log_error "  Failed to obtain credentials for cluster '$cluster_name'."
                    fi
                fi
            fi
        done <<< "$clusters_output"
    done

    # Final summary report
    log_header "Synchronization Summary"
    echo -e "  ${COLOR_BOLD}Total projects scanned:${COLOR_RESET}        $total_valid (Excluded sys-*: $excluded_sys_count)"
    echo -e "  ${COLOR_BOLD}Projects with GKE:${COLOR_RESET}             $projects_with_gke"
    echo -e "  ${COLOR_BOLD}Projects with errors/no API:${COLOR_RESET}   $projects_with_errors"
    echo -e "  ${COLOR_BOLD}Total clusters found:${COLOR_RESET}          $total_clusters_found"
    echo -e "  ${COLOR_BOLD}Clusters added/updated:${COLOR_RESET}        ${COLOR_GREEN}$clusters_added${COLOR_RESET}"
    echo -e "  ${COLOR_BOLD}Clusters skipped (duplicates):${COLOR_RESET} ${COLOR_YELLOW}$clusters_skipped${COLOR_RESET}"
    echo ""

    if [[ "$DRY_RUN" == true ]]; then
        log_info "Dry-run completed. Run the script without '-d' to apply changes to kubeconfig."
    else
        log_success "Synchronization completed successfully."
    fi
}

main "$@"
