// Script to create test data for the Pixelog system
const fs = require('fs')
const path = require('path')

// Create mock PixeFile data that matches our API structure
const testFiles = [
  {
    id: 'pixe_001',
    name: 'vacation-photos-2024.pixe',
    size: '2.4 MB',
    path: '/converted/vacation-photos-2024.pixe',
    created_at: '2024-07-22T10:30:00Z',
    metadata: {
      original_files: ['IMG_001.jpg', 'IMG_002.jpg', 'IMG_003.jpg'],
      compression_ratio: 0.85,
      qr_code_count: 1240,
      video_duration: 31.5,
      frame_rate: 30,
      encoding_format: 'MP4',
      checksum: 'sha256:a1b2c3d4...'
    }
  },
  {
    id: 'pixe_002', 
    name: 'tax-documents-2024.pixe',
    size: '1.8 MB',
    path: '/converted/tax-documents-2024.pixe', 
    created_at: '2024-04-10T14:20:00Z',
    metadata: {
      original_files: ['W2_2024.pdf', '1099_forms.pdf', 'receipts.zip'],
      compression_ratio: 0.92,
      qr_code_count: 890,
      video_duration: 22.8,
      frame_rate: 30,
      encoding_format: 'MP4',
      checksum: 'sha256:e5f6g7h8...'
    }
  },
  {
    id: 'pixe_003',
    name: 'meeting-notes-project-alpha.pixe', 
    size: '0.5 MB',
    path: '/converted/meeting-notes-project-alpha.pixe',
    created_at: '2024-09-15T16:45:00Z',
    metadata: {
      original_files: ['meeting_notes.docx'],
      compression_ratio: 0.78,
      qr_code_count: 235,
      video_duration: 6.2,
      frame_rate: 30,
      encoding_format: 'MP4', 
      checksum: 'sha256:i9j0k1l2...'
    }
  },
  {
    id: 'pixe_004',
    name: 'family-recipes-collection.pixe',
    size: '0.3 MB', 
    path: '/converted/family-recipes-collection.pixe',
    created_at: '2024-06-08T12:15:00Z',
    metadata: {
      original_files: ['recipes.txt'],
      compression_ratio: 0.65,
      qr_code_count: 142,
      video_duration: 3.8,
      frame_rate: 30,
      encoding_format: 'MP4',
      checksum: 'sha256:m3n4o5p6...'
    }
  },
  {
    id: 'pixe_005',
    name: 'code-backup-frontend.pixe',
    size: '3.1 MB',
    path: '/converted/code-backup-frontend.pixe', 
    created_at: '2024-09-20T09:30:00Z',
    metadata: {
      original_files: ['src/', 'components/', 'utils/'],
      compression_ratio: 0.88,
      qr_code_count: 1580,
      video_duration: 39.2,
      frame_rate: 30,
      encoding_format: 'MP4',
      checksum: 'sha256:q7r8s9t0...'
    }
  },
  {
    id: 'pixe_006', 
    name: 'quarterly-report-Q3-2024.pixe',
    size: '1.2 MB',
    path: '/converted/quarterly-report-Q3-2024.pixe',
    created_at: '2024-09-28T17:00:00Z', 
    metadata: {
      original_files: ['Q3_report.xlsx', 'charts.png'],
      compression_ratio: 0.81,
      qr_code_count: 612,
      video_duration: 15.3,
      frame_rate: 30,
      encoding_format: 'MP4',
      checksum: 'sha256:u1v2w3x4...'
    }
  }
]

// Save test data to JSON file
const outputPath = path.join(__dirname, 'test-data.json')
fs.writeFileSync(outputPath, JSON.stringify(testFiles, null, 2))

console.log(`Created ${testFiles.length} test files:`)
testFiles.forEach(file => {
  console.log(`- ${file.name} (${file.size})`)
})
console.log(`\nTest data saved to: ${outputPath}`)
