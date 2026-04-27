package index

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type DeltaManager struct {
	deltaDir string
	indexer  *Indexer
}

func NewDeltaManager(deltaDir string, indexer *Indexer) (*DeltaManager, error) {
	if err := os.MkdirAll(deltaDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create delta directory: %w", err)
	}
	
	return &DeltaManager{
		deltaDir: deltaDir,
		indexer:  indexer,
	}, nil
}

// CreateVersion creates a new version with delta encoding
func (dm *DeltaManager) CreateVersion(memoryID string, newPixeFile string, message string, author string) (*DeltaVersion, error) {
	// Load versioned memory or create new one
	vm, err := dm.LoadVersionedMemory(memoryID)
	if os.IsNotExist(err) {
		// First version - create base
		vm = &VersionedMemory{
			MemoryID:    memoryID,
			BaseFile:    newPixeFile,
			CurrentHead: 1,
			Versions:    []DeltaVersion{},
			Branches:    map[string]int{"main": 1},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		return nil, dm.SaveVersionedMemory(vm)
	} else if err != nil {
		return nil, err
	}
	
	// Create new version
	newVersion := vm.CurrentHead + 1
	
	// Calculate delta operations
	ops, err := dm.calculateDelta(vm, newPixeFile)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate delta: %w", err)
	}
	
	// Create delta version
	deltaVer := DeltaVersion{
		Version:       newVersion,
		ParentVersion: vm.CurrentHead,
		Timestamp:     time.Now(),
		DeltaFile:     newPixeFile,
		Operations:    ops,
		Message:       message,
		Author:        author,
		FrameCount:    len(ops),
	}
	
	vm.Versions = append(vm.Versions, deltaVer)
	vm.CurrentHead = newVersion
	vm.UpdatedAt = time.Now()
	vm.Branches["main"] = newVersion
	
	if err := dm.SaveVersionedMemory(vm); err != nil {
		return nil, err
	}
	
	return &deltaVer, nil
}

// calculateDelta compares old and new versions to find differences
func (dm *DeltaManager) calculateDelta(vm *VersionedMemory, newPixeFile string) ([]DeltaOp, error) {
	// For MVP, we'll do a simple approach:
	// Extract both versions and compare content hashes
	
	// TODO: Implement smart diff algorithm
	// For now, return a simple "replace all" operation
	ops := []DeltaOp{
		{
			Type:       "replace_all",
			FrameIndex: 0,
			OldHash:    "",
			NewHash:    dm.hashFile(newPixeFile),
			ChunkData:  "", // Will be stored in delta file
		},
	}
	
	return ops, nil
}

// ReconstructVersion reconstructs content at a specific version
func (dm *DeltaManager) ReconstructVersion(memoryID string, version int) (string, error) {
	vm, err := dm.LoadVersionedMemory(memoryID)
	if err != nil {
		return "", err
	}
	
	if version == 1 {
		// Return base version
		return vm.BaseFile, nil
	}
	
	// Apply deltas up to target version
	currentFile := vm.BaseFile
	for i := 0; i < len(vm.Versions) && vm.Versions[i].Version <= version; i++ {
		delta := vm.Versions[i]
		currentFile = delta.DeltaFile // Simplified for MVP
	}
	
	return currentFile, nil
}

// CreateBranch creates a new branch from a specific version
func (dm *DeltaManager) CreateBranch(memoryID string, branchName string, fromVersion int) error {
	vm, err := dm.LoadVersionedMemory(memoryID)
	if err != nil {
		return err
	}
	
	if _, exists := vm.Branches[branchName]; exists {
		return fmt.Errorf("branch %s already exists", branchName)
	}
	
	vm.Branches[branchName] = fromVersion
	vm.UpdatedAt = time.Now()
	
	return dm.SaveVersionedMemory(vm)
}

// GetVersionDiff returns the differences between two versions
func (dm *DeltaManager) GetVersionDiff(memoryID string, fromVersion, toVersion int) ([]DeltaOp, error) {
	vm, err := dm.LoadVersionedMemory(memoryID)
	if err != nil {
		return nil, err
	}
	
	var allOps []DeltaOp
	for i := fromVersion; i < toVersion && i <= len(vm.Versions); i++ {
		if i > 0 && i <= len(vm.Versions) {
			allOps = append(allOps, vm.Versions[i-1].Operations...)
		}
	}
	
	return allOps, nil
}

// LoadVersionedMemory loads version history from disk
func (dm *DeltaManager) LoadVersionedMemory(memoryID string) (*VersionedMemory, error) {
	versionPath := filepath.Join(dm.deltaDir, memoryID+".versions")
	data, err := os.ReadFile(versionPath)
	if err != nil {
		return nil, err
	}
	
	var vm VersionedMemory
	if err := json.Unmarshal(data, &vm); err != nil {
		return nil, fmt.Errorf("failed to parse version history: %w", err)
	}
	
	return &vm, nil
}

// SaveVersionedMemory saves version history to disk
func (dm *DeltaManager) SaveVersionedMemory(vm *VersionedMemory) error {
	versionPath := filepath.Join(dm.deltaDir, vm.MemoryID+".versions")
	data, err := json.MarshalIndent(vm, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal version history: %w", err)
	}
	
	if err := os.WriteFile(versionPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write version history: %w", err)
	}
	
	return nil
}

// ListVersions returns all versions for a memory
func (dm *DeltaManager) ListVersions(memoryID string) ([]DeltaVersion, error) {
	vm, err := dm.LoadVersionedMemory(memoryID)
	if err != nil {
		return nil, err
	}
	
	return vm.Versions, nil
}

// ListBranches returns all branches for a memory
func (dm *DeltaManager) ListBranches(memoryID string) (map[string]int, error) {
	vm, err := dm.LoadVersionedMemory(memoryID)
	if err != nil {
		return nil, err
	}
	
	return vm.Branches, nil
}

func (dm *DeltaManager) hashFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256(data))
}
