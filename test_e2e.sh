#!/bin/bash
set -e

echo "╔════════════════════════════════════════════════════════════╗"
echo "║         PIXELOG E2E TEST - Complete Workflow              ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test file
TEST_FILE="test_e2e_doc.txt"
PIXE_FILE="test_e2e.pixe"

echo "📝 Creating test document..."
cat > $TEST_FILE << 'EOF'
PIXELOG E2E TEST DOCUMENT

This document tests the complete Pixelog workflow including:

1. Conversion to .pixe format
2. Smart indexing with vector embeddings
3. Semantic search capabilities
4. Interactive LLM chat
5. Version control and time-travel queries

The main topics covered are:
- File archival using QR codes
- Delta encoding for efficient storage
- Offline-first design philosophy
- Semantic search without servers

Key features:
- Sub-100ms retrieval times
- Git-like version control
- Works completely air-gapped
- Pure Go implementation
EOF

echo -e "${GREEN}✓${NC} Test document created"
echo ""

# Step 1: Convert
echo -e "${BLUE}════ Step 1: Convert to .pixe format ════${NC}"
./pixe convert $TEST_FILE -o $PIXE_FILE
echo -e "${GREEN}✓${NC} Conversion complete"
echo ""

# Step 2: Info
echo -e "${BLUE}════ Step 2: File information ════${NC}"
./pixe info $PIXE_FILE
echo ""

# Step 3: Verify
echo -e "${BLUE}════ Step 3: Verify integrity ════${NC}"
./pixe verify $PIXE_FILE
echo ""

# Step 4: Index
echo -e "${BLUE}════ Step 4: Build search index ════${NC}"
./pixe index $PIXE_FILE
echo -e "${GREEN}✓${NC} Index built"
echo ""

# Step 5: Search
echo -e "${BLUE}════ Step 5: Semantic search ════${NC}"
echo "Query: 'main topics'"
./pixe search $PIXE_FILE "main topics" --top 3
echo ""

# Step 6: Version control
echo -e "${BLUE}════ Step 6: Version control ════${NC}"
./pixe version $PIXE_FILE -m "Initial version" --author "e2e-test"
echo ""

# Modify file
cat >> $TEST_FILE << 'EOF'

UPDATED SECTION:
This is a new section added in version 2.
It demonstrates delta encoding capabilities.
EOF

echo "Modified document, creating v2..."
./pixe convert $TEST_FILE -o $PIXE_FILE
./pixe version $PIXE_FILE -m "Added new section" --author "e2e-test"
echo ""

# List versions
echo "Listing all versions:"
./pixe versions $PIXE_FILE
echo ""

# Step 7: Chat (if API key available)
echo -e "${BLUE}════ Step 7: Interactive chat test ════${NC}"
if [ -n "$OPENROUTER_API_KEY" ] || [ -n "$OPENAI_API_KEY" ]; then
    echo "API key found, testing chat..."
    echo -e "${YELLOW}Note: Chat is interactive, skipping in automated test${NC}"
    echo "To test manually: ./pixe chat $PIXE_FILE --api-key YOUR_KEY"
else
    echo -e "${YELLOW}⚠ No API key found${NC}"
    echo "Set OPENROUTER_API_KEY or OPENAI_API_KEY to test chat"
    echo "Example: export OPENROUTER_API_KEY=sk-or-v1-xxxxx"
fi
echo ""

# Summary
echo "╔════════════════════════════════════════════════════════════╗"
echo "║                  E2E TEST COMPLETE                         ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""
echo -e "${GREEN}✓ All steps completed successfully!${NC}"
echo ""
echo "Test coverage:"
echo "  ✓ File conversion (.txt → .pixe)"
echo "  ✓ File information (info command)"
echo "  ✓ Integrity verification (all QR codes)"
echo "  ✓ Vector index building"
echo "  ✓ Semantic search (mock embedder)"
echo "  ✓ Version control (v1 → v2)"
echo "  ✓ Version listing"
echo ""
echo "To test chat: ./pixe chat $PIXE_FILE --api-key YOUR_KEY"
echo "Example queries:"
echo "  - 'What are the main topics?'"
echo "  - 'Tell me about delta encoding'"
echo "  - 'How fast is retrieval?'"
echo ""

# Cleanup option
read -p "Clean up test files? (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    rm -f $TEST_FILE $PIXE_FILE
    rm -rf indexes/ deltas/
    echo -e "${GREEN}✓${NC} Cleaned up"
fi
