// @vitest-environment jsdom
import {afterEach, expect, test, vi} from 'vitest'
import {cleanup, fireEvent, render, screen} from '@testing-library/react'
import {ListCatalog} from '../wailsjs/go/main/App'
import App from './App'

vi.mock('../wailsjs/go/main/App', () => ({ListCatalog: vi.fn()}))
afterEach(() => { cleanup(); vi.resetAllMocks() })

test('renders the backend result after loading', async () => {
  vi.mocked(ListCatalog).mockResolvedValue([{id: 'test', name: 'Fixture from Go', category: 'Relic'}])
  render(<App />)
  expect(screen.getByRole('status').textContent).toContain('Loading')
  expect(await screen.findByText('Fixture from Go')).toBeTruthy()
  expect(screen.getByText('Demo data')).toBeTruthy()
})

test('offers retry after failure and shows an empty result', async () => {
  vi.mocked(ListCatalog).mockRejectedValueOnce(new Error('unavailable')).mockResolvedValueOnce([])
  render(<App />)
  await screen.findByRole('alert')
  fireEvent.click(screen.getByRole('button', {name: 'Retry'}))
  expect(await screen.findByText('No examples available.')).toBeTruthy()
  expect(ListCatalog).toHaveBeenCalledTimes(2)
})
