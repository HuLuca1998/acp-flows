import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach } from 'vitest'

// 每个测试之间清干净 DOM——残留的节点会让 getByRole 查到上一个测试的元素，
// 症状是「单跑绿、全跑红」，很难排查。
afterEach(cleanup)
