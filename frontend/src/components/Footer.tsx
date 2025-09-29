import React, { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X } from 'lucide-react'

// ===== COMPONENT TYPES =====

interface FooterProps {
  readonly className?: string
}

interface ModalProps {
  readonly isOpen: boolean
  readonly onClose: () => void
  readonly title: string
  readonly children: React.ReactNode
}

// ===== MODAL COMPONENT =====

const Modal: React.FC<ModalProps> = ({ isOpen, onClose, title, children }) => {
  return (
    <AnimatePresence>
      {isOpen && (
        <motion.div
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          className="fixed inset-0 z-50 bg-black/50 backdrop-blur-sm flex items-center justify-center p-4"
          onClick={onClose}
        >
          <motion.div
            initial={{ opacity: 0, scale: 0.9 }}
            animate={{ opacity: 1, scale: 1 }}
            exit={{ opacity: 0, scale: 0.9 }}
            className="cyber-terminal max-w-2xl w-full max-h-[80vh] overflow-auto"
            onClick={e => e.stopPropagation()}
          >
            <div className="cyber-terminal-header">
              <h2 className="cyber-h2 text-lg flex-1">{title}</h2>
              <button
                onClick={onClose}
                className="p-2 cyber-text-secondary hover:cyber-text-primary transition-colors"
                aria-label="Close modal"
              >
                <X className="w-5 h-5" />
              </button>
            </div>
            <div className="cyber-terminal-body">
              {children}
            </div>
          </motion.div>
        </motion.div>
      )}
    </AnimatePresence>
  )
}

// ===== FOOTER COMPONENT =====

const Footer: React.FC<FooterProps> = ({ className = '' }) => {
  const [privacyOpen, setPrivacyOpen] = useState<boolean>(false)
  const [termsOpen, setTermsOpen] = useState<boolean>(false)

  return (
    <>
      <footer className={`cyber-bg-panel mt-16 py-6 ${className}`}>
        <div className="container mx-auto px-4">
          <div className="flex flex-col sm:flex-row justify-between items-center gap-4">
            {/* Left side - Copyright */}
            <div className="cyber-mono text-sm cyber-text-secondary">
              © 2025 Pixelog • Open Source • MIT License
            </div>
            
            {/* Right side - Links */}
            <div className="flex items-center space-x-6">
              <button
                onClick={() => setPrivacyOpen(true)}
                className="cyber-mono text-sm cyber-text-secondary hover:cyber-text-primary transition-colors"
              >
                Privacy
              </button>
              <span className="cyber-text-tertiary">|</span>
              <button
                onClick={() => setTermsOpen(true)}
                className="cyber-mono text-sm cyber-text-secondary hover:cyber-text-primary transition-colors"
              >
                Terms
              </button>
            </div>
          </div>
        </div>
      </footer>

      {/* Privacy Modal */}
      <Modal
        isOpen={privacyOpen}
        onClose={() => setPrivacyOpen(false)}
        title="Privacy Policy"
      >
        <div className="space-y-4">
          <div>
            <h3 className="cyber-h3 cyber-text-primary mb-2">Data Collection</h3>
            <p className="cyber-body cyber-text-secondary">
              Pixelog processes files locally on your device. No files or personal data are transmitted to external servers during the conversion process.
            </p>
          </div>
          
          <div>
            <h3 className="cyber-h3 cyber-text-primary mb-2">File Processing</h3>
            <p className="cyber-body cyber-text-secondary">
              All file conversions happen entirely within your browser or local application. Your files remain on your device throughout the entire process.
            </p>
          </div>
          
          <div>
            <h3 className="cyber-h3 cyber-text-primary mb-2">Storage</h3>
            <p className="cyber-body cyber-text-secondary">
              Generated .pixe files are stored locally on your device. No cloud storage or external servers are used unless you explicitly choose to upload files elsewhere.
            </p>
          </div>
          
          <div>
            <h3 className="cyber-h3 cyber-text-primary mb-2">Analytics</h3>
            <p className="cyber-body cyber-text-secondary">
              This application does not collect usage analytics or tracking data. Your privacy is fully protected.
            </p>
          </div>
        </div>
      </Modal>

      {/* Terms Modal */}
      <Modal
        isOpen={termsOpen}
        onClose={() => setTermsOpen(false)}
        title="Terms of Service"
      >
        <div className="space-y-4">
          <div>
            <h3 className="cyber-h3 cyber-text-primary mb-2">MIT License</h3>
            <p className="cyber-body cyber-text-secondary">
              Pixelog is released under the MIT License. You are free to use, modify, and distribute this software for any purpose, commercial or non-commercial.
            </p>
          </div>
          
          <div>
            <h3 className="cyber-h3 cyber-text-primary mb-2">Warranty Disclaimer</h3>
            <p className="cyber-body cyber-text-secondary">
              This software is provided "as is" without warranty of any kind. The authors are not liable for any damages arising from the use of this software.
            </p>
          </div>
          
          <div>
            <h3 className="cyber-h3 cyber-text-primary mb-2">User Responsibility</h3>
            <p className="cyber-body cyber-text-secondary">
              Users are responsible for ensuring they have the right to convert and distribute any files they process with Pixelog.
            </p>
          </div>
          
          <div>
            <h3 className="cyber-h3 cyber-text-primary mb-2">Open Source</h3>
            <p className="cyber-body cyber-text-secondary">
              The complete source code is available on GitHub. Contributions and feedback are welcome from the community.
            </p>
          </div>
        </div>
      </Modal>
    </>
  )
}

export default Footer
