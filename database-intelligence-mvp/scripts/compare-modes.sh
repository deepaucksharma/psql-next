#!/bin/bash
# Interactive mode comparison tool for Database Intelligence MVP
# Helps users decide between Standard and Experimental modes

set -euo pipefail

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# User answers
ANSWERS=()
SCORE_STANDARD=0
SCORE_EXPERIMENTAL=0

# Questions and scoring
ask_question() {
    local question="$1"
    local option_a="$2"
    local option_b="$3"
    local score_a="$4"
    local score_b="$5"
    
    echo ""
    echo -e "${CYAN}$question${NC}"
    echo -e "  ${BOLD}A)${NC} $option_a"
    echo -e "  ${BOLD}B)${NC} $option_b"
    echo ""
    
    while true; do
        read -p "Your choice (A/B): " choice
        case "${choice,,}" in
            a)
                ANSWERS+=("$option_a")
                ((SCORE_STANDARD += score_a))
                ((SCORE_EXPERIMENTAL += score_b))
                break
                ;;
            b)
                ANSWERS+=("$option_b")
                ((SCORE_STANDARD += score_b))
                ((SCORE_EXPERIMENTAL += score_a))
                break
                ;;
            *)
                echo "Please enter A or B"
                ;;
        esac
    done
}

# Display feature comparison table
show_feature_table() {
    echo ""
    echo -e "${BOLD}Feature Comparison:${NC}"
    echo ""
    printf "%-30s %-20s %-20s\n" "Feature" "Standard" "Experimental"
    printf "%-30s %-20s %-20s\n" "------------------------------" "--------------------" "--------------------"
    
    # Features array: "Feature name|Standard|Experimental"
    local features=(
        "Build Required|❌ No|✅ Yes"
        "Production Ready|✅ Yes|⚠️  Beta"
        "Resource Usage|✅ Low (512MB)|⚠️  Higher (2GB)"
        "High Availability|✅ 3 replicas|❌ Single instance"
        "Setup Time|✅ 5 minutes|⚠️  15-20 minutes"
        "|${GREEN}━━━━━━━━━━━━━━━━━━━━${NC}|${PURPLE}━━━━━━━━━━━━━━━━━━━━${NC}"
        "Query Metadata|✅ Yes|✅ Yes"
        "Performance Metrics|✅ Yes|✅ Yes"
        "PII Sanitization|✅ Yes|✅ Yes"
        "Sampling|✅ Fixed 25%|✅ Adaptive"
        "|${GREEN}━━━━━━━━━━━━━━━━━━━━${NC}|${PURPLE}━━━━━━━━━━━━━━━━━━━━${NC}"
        "ASH (1-sec samples)|❌ No|✅ Yes"
        "Circuit Breaker|❌ No|✅ Yes"
        "Multi-Database|⚠️  Manual|✅ Native"
        "Cloud Optimization|❌ No|✅ Yes"
        "Plan Analysis Ready|❌ No|✅ Yes"
    )
    
    for feature in "${features[@]}"; do
        IFS='|' read -r name standard experimental <<< "$feature"
        if [[ "$name" == "" ]]; then
            # Separator line
            printf "%-30s %-20s %-20s\n" "$standard" "$experimental" ""
        else
            printf "%-30s %-20s %-20s\n" "$name" "$standard" "$experimental"
        fi
    done
}

# Resource comparison
show_resource_comparison() {
    echo ""
    echo -e "${BOLD}Resource Requirements:${NC}"
    echo ""
    
    # Standard Mode
    echo -e "${GREEN}Standard Mode:${NC}"
    echo "  • Memory: 256-512MB per instance"
    echo "  • CPU: 100-300m"
    echo "  • Disk: 100MB"
    echo "  • Network: <1Mbps"
    echo "  • Instances: 3 (for HA)"
    echo ""
    
    # Experimental Mode
    echo -e "${PURPLE}Experimental Mode:${NC}"
    echo "  • Memory: 1-2GB"
    echo "  • CPU: 500m-1000m"
    echo "  • Disk: 500MB"
    echo "  • Network: 1-5Mbps"
    echo "  • Instances: 1 (stateful)"
}

# Show use cases
show_use_cases() {
    echo ""
    echo -e "${BOLD}Recommended Use Cases:${NC}"
    echo ""
    
    echo -e "${GREEN}Choose Standard Mode for:${NC}"
    echo "  ✓ Production databases"
    echo "  ✓ Compliance environments"
    echo "  ✓ Resource-constrained systems"
    echo "  ✓ Quick deployment needs"
    echo "  ✓ Proven stability requirements"
    echo ""
    
    echo -e "${PURPLE}Choose Experimental Mode for:${NC}"
    echo "  ✓ Performance troubleshooting"
    echo "  ✓ Development/staging environments"
    echo "  ✓ Advanced monitoring needs"
    echo "  ✓ Multi-database environments"
    echo "  ✓ Future-proofing deployments"
}

# Interactive questionnaire
run_questionnaire() {
    echo ""
    echo -e "${BOLD}=== Interactive Mode Selection ===${NC}"
    echo ""
    echo "Answer a few questions to help determine the best mode for your needs."
    
    # Question 1: Environment
    ask_question \
        "1. What type of environment are you monitoring?" \
        "Production database" \
        "Development/Staging/Testing" \
        3 0
    
    # Question 2: Monitoring needs
    ask_question \
        "2. What are your primary monitoring needs?" \
        "Basic metrics and query performance" \
        "Detailed troubleshooting and analysis" \
        3 0
    
    # Question 3: Resource availability
    ask_question \
        "3. How much memory can you allocate?" \
        "Limited (< 1GB)" \
        "Plenty (2GB+)" \
        3 0
    
    # Question 4: Session visibility
    ask_question \
        "4. Do you need second-by-second session visibility?" \
        "No, 5-minute intervals are fine" \
        "Yes, I need ASH-like monitoring" \
        3 0
    
    # Question 5: Database protection
    ask_question \
        "5. Do you need automatic database protection?" \
        "No, I'll monitor manually" \
        "Yes, circuit breaker would be helpful" \
        2 1
    
    # Question 6: Multiple databases
    ask_question \
        "6. How many databases will you monitor?" \
        "Just one or two" \
        "Multiple databases (3+)" \
        2 1
    
    # Question 7: Build capability
    ask_question \
        "7. Can you build custom software?" \
        "Prefer pre-built only" \
        "Yes, building is fine" \
        3 0
    
    # Question 8: Time to deploy
    ask_question \
        "8. How quickly do you need to deploy?" \
        "ASAP (< 10 minutes)" \
        "I can spend 30+ minutes" \
        3 0
}

# Calculate and show results
show_results() {
    echo ""
    echo -e "${BOLD}=== Results ===${NC}"
    echo ""
    
    # Show scores
    echo "Based on your answers:"
    echo -e "  Standard Mode Score: ${GREEN}$SCORE_STANDARD${NC}"
    echo -e "  Experimental Mode Score: ${PURPLE}$SCORE_EXPERIMENTAL${NC}"
    echo ""
    
    # Recommendation
    if [[ $SCORE_STANDARD -gt $((SCORE_EXPERIMENTAL + 5)) ]]; then
        echo -e "${BOLD}${GREEN}Recommendation: Standard Mode${NC}"
        echo ""
        echo "Standard mode is the best fit for your requirements. It provides:"
        echo "  • Production-ready stability"
        echo "  • Low resource usage"
        echo "  • Quick deployment"
        echo "  • Proven reliability"
        echo ""
        echo -e "${BOLD}Deploy with:${NC}"
        echo "  ./quickstart.sh all"
        
    elif [[ $SCORE_EXPERIMENTAL -gt $((SCORE_STANDARD + 5)) ]]; then
        echo -e "${BOLD}${PURPLE}Recommendation: Experimental Mode${NC}"
        echo ""
        echo "Experimental mode matches your needs better. It offers:"
        echo "  • Advanced monitoring features"
        echo "  • Active Session History"
        echo "  • Intelligent sampling"
        echo "  • Database protection"
        echo ""
        echo -e "${BOLD}Deploy with:${NC}"
        echo "  ./quickstart.sh --experimental all"
        
    else
        echo -e "${BOLD}${YELLOW}Recommendation: Consider Both${NC}"
        echo ""
        echo "Your needs could be met by either mode. Consider:"
        echo ""
        echo "  • Start with Standard if you need production stability"
        echo "  • Choose Experimental if you want advanced features"
        echo "  • You can run both side-by-side for comparison"
        echo ""
        echo -e "${BOLD}Deploy Standard:${NC} ./quickstart.sh all"
        echo -e "${BOLD}Deploy Experimental:${NC} ./quickstart.sh --experimental all"
    fi
}

# Main menu
show_menu() {
    while true; do
        clear
        echo -e "${BOLD}Database Intelligence MVP - Mode Comparison Tool${NC}"
        echo ""
        echo "This tool helps you choose between Standard and Experimental deployment modes."
        echo ""
        echo "1) View Feature Comparison Table"
        echo "2) View Resource Requirements"
        echo "3) View Recommended Use Cases"
        echo "4) Take Interactive Questionnaire"
        echo "5) View Architecture Diagrams"
        echo "6) Exit"
        echo ""
        read -p "Select an option (1-6): " choice
        
        case $choice in
            1)
                show_feature_table
                echo ""
                read -p "Press Enter to continue..."
                ;;
            2)
                show_resource_comparison
                echo ""
                read -p "Press Enter to continue..."
                ;;
            3)
                show_use_cases
                echo ""
                read -p "Press Enter to continue..."
                ;;
            4)
                SCORE_STANDARD=0
                SCORE_EXPERIMENTAL=0
                ANSWERS=()
                run_questionnaire
                show_results
                echo ""
                read -p "Press Enter to continue..."
                ;;
            5)
                echo ""
                echo "Generating architecture diagrams..."
                if [[ -x "${SCRIPT_DIR}/generate-architecture-diagram.py" ]]; then
                    python3 "${SCRIPT_DIR}/generate-architecture-diagram.py"
                    echo ""
                    echo "Opening diagrams..."
                    if [[ -f "${PROJECT_ROOT}/docs/architecture-diagrams.html" ]]; then
                        open "${PROJECT_ROOT}/docs/architecture-diagrams.html" 2>/dev/null || \
                        xdg-open "${PROJECT_ROOT}/docs/architecture-diagrams.html" 2>/dev/null || \
                        echo "Please open: ${PROJECT_ROOT}/docs/architecture-diagrams.html"
                    fi
                else
                    echo "Architecture diagram script not found"
                fi
                echo ""
                read -p "Press Enter to continue..."
                ;;
            6)
                echo ""
                echo "Ready to deploy? Use one of these commands:"
                echo ""
                echo -e "  ${GREEN}Standard Mode:${NC} ./quickstart.sh all"
                echo -e "  ${PURPLE}Experimental Mode:${NC} ./quickstart.sh --experimental all"
                echo ""
                exit 0
                ;;
            *)
                echo "Invalid option. Please select 1-6."
                read -p "Press Enter to continue..."
                ;;
        esac
    done
}

# Quick comparison if --quick flag is used
if [[ "${1:-}" == "--quick" ]]; then
    show_feature_table
    echo ""
    echo -e "${BOLD}Quick Recommendation:${NC}"
    echo -e "  • ${GREEN}Production?${NC} → Use Standard Mode"
    echo -e "  • ${PURPLE}Advanced monitoring?${NC} → Use Experimental Mode"
    echo -e "  • ${YELLOW}Not sure?${NC} → Run without --quick for interactive guide"
    echo ""
    exit 0
fi

# Show help
if [[ "${1:-}" == "--help" ]] || [[ "${1:-}" == "-h" ]]; then
    echo "Usage: $0 [--quick]"
    echo ""
    echo "Interactive tool to help choose between Standard and Experimental modes."
    echo ""
    echo "Options:"
    echo "  --quick   Show quick comparison table and exit"
    echo "  --help    Show this help message"
    echo ""
    exit 0
fi

# Run interactive menu
show_menu